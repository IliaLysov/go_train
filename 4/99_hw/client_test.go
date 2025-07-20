package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const AccessToken = "token"

type TestCase struct {
	SearchRequest
	SearchResponse
	IsError     bool
	AccessToken string
}
type UserXML struct {
	Id        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

type RootXML struct {
	Version string    `xml:"version,attr"`
	List    []UserXML `xml:"row"`
}

func JoinAndFilter(list []UserXML, query string) []User {
	var res []User
	for _, user := range list {
		name := user.FirstName + " " + user.LastName
		if query == "" || strings.Contains(name, query) || strings.Contains(user.About, query) {
			res = append(res, User{
				Id:     user.Id,
				Name:   name,
				Age:    user.Age,
				About:  user.About,
				Gender: user.Gender,
			})
		}
	}
	return res
}

func Sort(users *[]User, orderField string, orderBy int) {
	if orderBy != 0 {
		sort.Slice(*users, func(i, j int) bool {
			ui := reflect.ValueOf((*users)[i]).FieldByName(orderField)
			uj := reflect.ValueOf((*users)[j]).FieldByName(orderField)

			if !ui.IsValid() || !uj.IsValid() {
				return false
			}

			switch ui.Kind() {
			case reflect.Int:
				if orderBy > 0 {
					return ui.Int() > uj.Int()
				}
				return ui.Int() < uj.Int()
			case reflect.String:
				if orderBy > 0 {
					return ui.String() > uj.String()
				}
				return ui.String() < uj.String()
			default:
				return false
			}
		})
	}
}

func SearchServer(w http.ResponseWriter, r *http.Request) {

	rAccessToken := r.Header.Get("AccessToken")
	if rAccessToken != AccessToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	query := r.URL.Query().Get("query")
	orderField := r.URL.Query().Get("order_field")
	orderBy, _ := strconv.Atoi(r.URL.Query().Get("order_by"))

	switch orderField {
	case "Id", "Age", "Name":
	default:
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error": "ErrorBadOrderField"}`)
		return
	}

	f, err := os.Open("dataset.xml")
	if err != nil {
		panic(err)
	}
	rt := new(RootXML)
	data, _ := io.ReadAll(f)
	err = xml.Unmarshal(data, &rt)
	if err != nil {
		panic(err)
	}
	users := JoinAndFilter(rt.List, query)
	Sort(&users, orderField, orderBy)
	if len(users) > offset+limit {
		users = users[offset : offset+limit]
	} else if offset < len(users) {
		users = users[offset:]
	} else {
		users = []User{}
	}
	res, err := json.Marshal(users)
	if err != nil {
		panic(err)
	}
	w.Write(res)
}

func TestClientFindUsers(t *testing.T) {
	cases := []TestCase{
		{
			SearchRequest: SearchRequest{
				Limit:      2,
				Offset:     2,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByDesc,
			},
			SearchResponse: SearchResponse{
				Users: []User{
					{
						Id:     32,
						Name:   "Christy Knapp",
						Age:    40,
						About:  "Incididunt culpa dolore laborum cupidatat consequat. Aliquip cupidatat pariatur sit consectetur laboris labore anim labore. Est sint ut ipsum dolor ipsum nisi tempor in tempor aliqua. Aliquip labore cillum est consequat anim officia non reprehenderit ex duis elit. Amet aliqua eu ad velit incididunt ad ut magna. Culpa dolore qui anim consequat commodo aute.\n",
						Gender: "female",
					},
					{
						Id:     31,
						Name:   "Palmer Scott",
						Age:    37,
						About:  "Elit fugiat commodo laborum quis eu consequat. In velit magna sit fugiat non proident ipsum tempor eu. Consectetur exercitation labore eiusmod occaecat adipisicing irure consequat fugiat ullamco aliquip nostrud anim irure enim. Duis do amet cillum eiusmod eu sunt. Minim minim sunt sit sit enim velit sint tempor enim sint aliquip voluptate reprehenderit officia. Voluptate magna sit consequat adipisicing ut eu qui.\n",
						Gender: "male",
					},
				},
				NextPage: true,
			},
			IsError:     false,
			AccessToken: AccessToken,
		},
		{
			SearchRequest: SearchRequest{
				Limit:      -1,
				Offset:     2,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByDesc,
			},
			IsError:     true,
			AccessToken: AccessToken,
		},
		{
			SearchRequest: SearchRequest{
				Limit:      1,
				Offset:     -1,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByDesc,
			},
			IsError:     true,
			AccessToken: AccessToken,
		},
		{
			SearchRequest: SearchRequest{
				Limit:      26,
				Offset:     20,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc,
			},
			IsError:     false,
			AccessToken: AccessToken,
		},
		{
			SearchRequest: SearchRequest{
				Limit:      26,
				Offset:     20,
				Query:      "",
				OrderField: "Id",
				OrderBy:    OrderByAsc,
			},
			IsError:     true,
			AccessToken: "wrong-token",
		},
		{
			SearchRequest: SearchRequest{
				Limit:      26,
				Offset:     20,
				Query:      "",
				OrderField: "Some",
				OrderBy:    OrderByAsc,
			},
			IsError:     true,
			AccessToken: AccessToken,
		},
		{
			SearchRequest: SearchRequest{
				Limit:      26,
				Offset:     20,
				Query:      "",
				OrderField: "Some",
				OrderBy:    OrderByAsc,
			},
			IsError:     true,
			AccessToken: AccessToken,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(SearchServer))

	for i, item := range cases {
		c := &SearchClient{
			URL:         ts.URL,
			AccessToken: item.AccessToken,
		}
		result, err := c.FindUsers(item.SearchRequest)

		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error %#v", i, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", i)
		}
		if result != nil && len(result.Users) > item.Limit {
			t.Errorf("[%d] wrong result, expected %d users, got %d", i, item.Limit, len(result.Users))
		}
		if len(item.SearchResponse.Users) > 0 && !reflect.DeepEqual(item.SearchResponse, *result) {
			t.Errorf("[%d] wrong result, expected %#v\n, got %#v\n", i, item.SearchResponse, *result)
		}
	}
	ts.Close()
}

func TestTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()
	c := &SearchClient{
		URL:         ts.URL,
		AccessToken: AccessToken,
	}
	_, err := c.FindUsers(SearchRequest{Limit: 1, Offset: 0, OrderField: "Id"})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestUnknownError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	c := &SearchClient{
		URL:         ts.URL,
		AccessToken: AccessToken,
	}
	_, err := c.FindUsers(SearchRequest{Limit: 1, Offset: 0, OrderField: "Id"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown error, got %v", err)
	}
}

func TestFatalError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := &SearchClient{
		URL:         ts.URL,
		AccessToken: AccessToken,
	}
	_, err := c.FindUsers(SearchRequest{Limit: 1, Offset: 0, OrderField: "Id"})
	if err == nil || !strings.Contains(err.Error(), "fatal") {
		t.Errorf("expected fatal error, got %v", err)
	}
}

func TestBadErrorJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("json"))
	}))
	defer ts.Close()

	c := &SearchClient{
		URL:         ts.URL,
		AccessToken: AccessToken,
	}
	_, err := c.FindUsers(SearchRequest{Limit: 1, Offset: 0, OrderField: "Id"})
	if err == nil || !strings.Contains(err.Error(), "unpack") {
		t.Errorf("expected json unpack error, got %v", err)
	}
}

func TestUnknownBadResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Error": "Unknown Error"}`))
	}))
	defer ts.Close()

	c := &SearchClient{
		URL:         ts.URL,
		AccessToken: AccessToken,
	}
	_, err := c.FindUsers(SearchRequest{Limit: 1, Offset: 0, OrderField: "Id"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown bad request error, got %v", err)
	}
}

func TestUnpackJSONError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`json`))
	}))
	defer ts.Close()

	c := &SearchClient{
		URL:         ts.URL,
		AccessToken: AccessToken,
	}
	_, err := c.FindUsers(SearchRequest{Limit: 1, Offset: 0, OrderField: "Id"})
	if err == nil || !strings.Contains(err.Error(), "unpack") {
		t.Errorf("expected unpack json error, got %v", err)
	}
}
