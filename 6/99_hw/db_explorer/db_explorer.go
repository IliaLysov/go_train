package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type Handler struct {
	db     *sql.DB
	tables map[string]Table
}

type Column struct {
	Name     string
	Type     string
	Nullable bool
}

type Table struct {
	Name    string
	Columns map[string]Column
	PriKey  string
}

type Where struct {
	Name  string
	Value string
}

type Query struct {
	Table  string
	Limit  int
	Offset int
	Id     string
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 {
		w.WriteHeader(status)
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handler) ok(w http.ResponseWriter, payload map[string]interface{}) {
	h.writeJSON(w, http.StatusOK, map[string]interface{}{"response": payload})
}

func (h *Handler) fail(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]interface{}{"error": msg})
}

func splitPath(r *http.Request) []string {
	p := strings.Trim(r.URL.Path, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func (h *Handler) tableOr404(w http.ResponseWriter, name string) (Table, bool) {
	t, ok := h.tables[name]
	if !ok {
		h.fail(w, http.StatusNotFound, "unknown table")
		return Table{}, false
	}
	return t, true
}

func intQuery(r *http.Request, key string) int {
	if v := r.FormValue(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?, ", n), ", ")
}

func backtickJoin(cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = fmt.Sprintf("`%s`", c)
	}
	return strings.Join(out, ", ")
}

func colNames(t Table) []string {
	names := make([]string, 0, len(t.Columns))
	for name := range t.Columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func coerce(val interface{}, col Column, missing bool) (interface{}, error) {
	if val == nil {
		if missing {
			if col.Nullable {
				return nil, nil
			}
			switch col.Type {
			case "int":
				return 0, nil
			default:
				return "", nil
			}
		}
		if !col.Nullable {
			return nil, fmt.Errorf("field %s have invalid type", col.Name)
		}
		return nil, nil
	}
	switch col.Type {
	case "int":
		switch v := val.(type) {
		case float64:
			return int(v), nil
		case string:
			if v == "" && col.Nullable {
				return nil, nil
			}
			i, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("field %s have invalid type", col.Name)
			}
			return i, nil
		case int:
			return v, nil
		default:
			return nil, fmt.Errorf("field %s have invalid type", col.Name)
		}
	default:
		if s, ok := val.(string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("field %s have invalid type", col.Name)
	}
}

func (h *Handler) GetTables(w http.ResponseWriter, r *http.Request) {
	var tables []string
	for key := range h.tables {
		tables = append(tables, key)
	}
	sort.Strings(tables)
	h.ok(w, map[string]interface{}{"tables": tables})
}

func (h *Handler) Query(q Query) (result []map[string]interface{}, err error) {
	t, ok := h.tables[q.Table]
	if !ok {
		return nil, fmt.Errorf("unknown table")
	}
	cols := colNames(t)
	args := []interface{}{}
	query := fmt.Sprintf("SELECT %s FROM `%s`", backtickJoin(cols), q.Table)
	if q.Id != "" {
		query += fmt.Sprintf(" WHERE %s = ?", h.tables[q.Table].PriKey)
		args = append(args, q.Id)
	}
	if q.Limit != 0 {
		query += " LIMIT ?"
		args = append(args, q.Limit)
	}
	if q.Offset != 0 {
		query += " OFFSET ?"
		args = append(args, q.Offset)
	}
	rows, err := h.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		row := make(map[string]interface{})
		values := make([]interface{}, len(h.tables[q.Table].Columns))
		valuePtrs := make([]interface{}, len(h.tables[q.Table].Columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err = rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				switch h.tables[q.Table].Columns[col].Type {
				case "int":
					row[col], err = strconv.Atoi(string(b))
					if err != nil {
						log.Fatal(err)
					}
				default:
					row[col] = string(b)
				}
			} else {
				row[col] = val
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func (h *Handler) GetRows(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r)
	tableName := parts[0]
	if _, ok := h.tableOr404(w, tableName); !ok {
		return
	}

	limit := intQuery(r, "limit")
	offset := intQuery(r, "offset")
	result, err := h.Query(Query{Limit: limit, Offset: offset, Table: tableName})
	if err != nil {
		h.fail(w, http.StatusBadRequest, err.Error())
		return
	}
	h.ok(w, map[string]interface{}{"records": result})
}

func (h *Handler) GetRow(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r)
	tableName, id := parts[0], parts[1]
	if _, ok := h.tableOr404(w, tableName); !ok {
		return
	}

	result, err := h.Query(Query{Id: id, Table: tableName})

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(result) < 1 {
		h.fail(w, http.StatusNotFound, "record not found")
		return
	}
	h.ok(w, map[string]interface{}{"record": result[0]})
}

func (h *Handler) PutRow(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r)
	tableName := parts[0]
	t, ok := h.tableOr404(w, tableName)
	if !ok {
		return
	}
	var values map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&values)
	if err != nil {
		h.fail(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Body.Close()
	var cols []string
	var args []interface{}

	for key, col := range t.Columns {
		if key == t.PriKey {
			continue
		}
		raw, exists := values[key]
		cv, err := coerce(raw, col, !exists)
		if err != nil {
			h.fail(w, http.StatusBadRequest, err.Error())
			return
		}
		cols = append(cols, key)
		args = append(args, cv)
	}
	query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", tableName, backtickJoin(cols), placeholders(len(args)))

	res, err := h.db.Exec(query, args...)
	if err != nil {
		h.fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	h.ok(w, map[string]interface{}{t.PriKey: id})
}

func (h *Handler) PostRow(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r)
	tableName, id := parts[0], parts[1]
	t, ok := h.tableOr404(w, tableName)
	if !ok {
		return
	}

	var values map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&values)
	if err != nil {
		h.fail(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Body.Close()
	var cols []string
	var args []interface{}

	if _, hasPk := values[t.PriKey]; hasPk {
		h.fail(w, http.StatusBadRequest, fmt.Sprintf("field %s have invalid type", t.PriKey))
		return
	}
	for colName, raw := range values {
		colMeta, ok := t.Columns[colName]
		if !ok || colName == t.PriKey {
			continue
		}
		cv, err := coerce(raw, colMeta, false)
		if err != nil {
			h.fail(w, http.StatusBadRequest, fmt.Sprintf("field %s have invalid type", colName))
			return
		}
		cols = append(cols, colName)
		args = append(args, cv)
	}
	args = append(args, id)
	places := strings.Join(cols, " = ?,") + " = ?"
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", tableName, places, t.PriKey)
	res, err := h.db.Exec(query, args...)
	if err != nil {
		h.fail(w, http.StatusBadRequest, err.Error())
		return
	}
	c, _ := res.RowsAffected()
	h.ok(w, map[string]interface{}{"updated": c})
}

func (h *Handler) DeleteRow(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r)
	tableName, id := parts[0], parts[1]
	t, ok := h.tableOr404(w, tableName)
	if !ok {
		return
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", tableName, t.PriKey)
	res, err := h.db.Exec(query, id)
	if err != nil {
		h.fail(w, http.StatusBadRequest, err.Error())
		return
	}
	c, _ := res.RowsAffected()
	h.ok(w, map[string]interface{}{"deleted": c})
}

func (h *Handler) Router(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r)

	switch {
	case len(parts) == 0:
		h.GetTables(w, r)
	case len(parts) == 1:
		if r.Method == http.MethodPut {
			h.PutRow(w, r)
		} else {
			h.GetRows(w, r)
		}
	case len(parts) == 2:
		switch r.Method {
		case http.MethodPost:
			h.PostRow(w, r)
		case http.MethodDelete:
			h.DeleteRow(w, r)
		default:
			h.GetRow(w, r)
		}
	default:
		h.fail(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) Init() {
	tables := make(map[string]Table)
	tableNames := []string{}
	rows, err := h.db.Query("SHOW TABLES")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			log.Fatal(err)
		}
		tableNames = append(tableNames, table)
	}

	for _, table := range tableNames {
		cTable := Table{
			Name:    table,
			Columns: make(map[string]Column),
		}

		crows, err := h.db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM `%s`", table))
		if err != nil {
			log.Fatal(err)
		}

		for crows.Next() {
			var (
				field, typ, key, extra, privileges string
				collation, nullStr, def, comment   sql.NullString
			)
			if err := crows.Scan(&field, &typ, &collation, &nullStr, &key, &def, &extra, &privileges, &comment); err != nil {
				log.Fatal(err)
			}
			if key == "PRI" {
				cTable.PriKey = field
			}
			var nullable bool
			if nullStr.String == "YES" {
				nullable = true
			}
			cTable.Columns[field] = Column{Name: field, Type: typ, Nullable: nullable}
		}
		crows.Close()
		tables[table] = cTable
	}

	h.tables = tables
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	handler := &Handler{
		db: db,
	}
	handler.Init()
	mux := http.NewServeMux()

	mux.HandleFunc("/", handler.Router)

	return mux, nil
}
