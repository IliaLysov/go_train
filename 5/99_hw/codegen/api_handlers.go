package main

import (
  "encoding/json"
  "net/http"
  "strconv"
)


func (api *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		api.handlerMyApiProfile(w, r)
	case "/user/create":
		api.handlerMyApiCreate(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown method",
		})
		return
	}
}

func (api *MyApi) handlerMyApiProfile(w http.ResponseWriter, r *http.Request) {

	var params ProfileParams
	//Login
	{
		paramName := "login"
		val := r.FormValue(paramName)
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "login must be not empty",
			})
			return
		}

		params.Login = val
	}

	res, err := api.Profile(r.Context(), params)
	if err != nil {
		switch e := err.(type) {
		case ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		case *ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":    "",
		"response": res,
	})
}

func (api *MyApi) handlerMyApiCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "bad method",
		})
		return
	}
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unauthorized",
		})
		return
	}

	var params CreateParams
	//Login
	{
		paramName := "login"
		val := r.FormValue(paramName)
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "login must be not empty",
			})
			return
		}
		if len(val) < 10 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "login len must be >= 10",
			})
			return
		}

		params.Login = val
	}
	//Name
	{
		paramName := "full_name"
		val := r.FormValue(paramName)

		params.Name = val
	}
	//Status
	{
		paramName := "status"
		val := r.FormValue(paramName)
		if val == "" {
			val = "user"
		}
		allowedStatus := map[string]bool{
			"user": true,
			"moderator": true,
			"admin": true,
		}
		if !allowedStatus[val] {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "status must be one of [user, moderator, admin]",
			})
			return
		}

		params.Status = val
	}
	//Age
	{
		paramName := "age"
		valStr := r.FormValue(paramName)

		var valInt int
		if valStr == "" {
			valInt = 0
		} else {
			tmp, err := strconv.Atoi(valStr)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "age must be int",
				})
				return
			}
			valInt = tmp
		}
		if valInt < 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "age must be >= 0",
			})
			return
		}
		if valInt > 128 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "age must be <= 128",
			})
			return
		}

		params.Age = valInt
	}

	res, err := api.Create(r.Context(), params)
	if err != nil {
		switch e := err.(type) {
		case ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		case *ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":    "",
		"response": res,
	})
}

func (api *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		api.handlerOtherApiCreate(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown method",
		})
		return
	}
}

func (api *OtherApi) handlerOtherApiCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "bad method",
		})
		return
	}
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unauthorized",
		})
		return
	}

	var params OtherCreateParams
	//Username
	{
		paramName := "username"
		val := r.FormValue(paramName)
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "username must be not empty",
			})
			return
		}
		if len(val) < 3 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "username len must be >= 3",
			})
			return
		}

		params.Username = val
	}
	//Name
	{
		paramName := "account_name"
		val := r.FormValue(paramName)

		params.Name = val
	}
	//Class
	{
		paramName := "class"
		val := r.FormValue(paramName)
		if val == "" {
			val = "warrior"
		}
		allowedClass := map[string]bool{
			"warrior": true,
			"sorcerer": true,
			"rouge": true,
		}
		if !allowedClass[val] {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "class must be one of [warrior, sorcerer, rouge]",
			})
			return
		}

		params.Class = val
	}
	//Level
	{
		paramName := "level"
		valStr := r.FormValue(paramName)

		var valInt int
		if valStr == "" {
			valInt = 0
		} else {
			tmp, err := strconv.Atoi(valStr)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "level must be int",
				})
				return
			}
			valInt = tmp
		}
		if valInt < 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "level must be >= 1",
			})
			return
		}
		if valInt > 50 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "level must be <= 50",
			})
			return
		}

		params.Level = valInt
	}

	res, err := api.Create(r.Context(), params)
	if err != nil {
		switch e := err.(type) {
		case ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		case *ApiError:
			w.WriteHeader(e.HTTPStatus)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": e.Err.Error(),
			})
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":    "",
		"response": res,
	})
}
