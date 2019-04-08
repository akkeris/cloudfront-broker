package broker

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
)

func HttpWrite(w http.ResponseWriter, status int, obj interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (b *BusinessLogic) addRoute(router *mux.Router, pathIn string, method string, handler func(string, map[string]string, *broker.RequestContext) (interface{}, error)) {
	path := fmt.Sprintf("/v2/service_instances/{instance_id}%s", pathIn)
	glog.Infof("AddRoute: Adding route %s", path)
	router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		c := broker.RequestContext{Request: r, Writer: w}
		obj, herr := handler(vars["instance_id"], vars, &c)
		type e struct {
			ErrorMessage *string `json:"error,omitempty"`
			Description  *string `json:"description,omitempty"`
		}
		if herr != nil {
			if httpErr, ok := osb.IsHTTPError(herr); ok {
				body := &e{}
				if httpErr.Description != nil {
					body.Description = httpErr.Description
				}
				if httpErr.ErrorMessage != nil {
					body.ErrorMessage = httpErr.ErrorMessage
				}
				HttpWrite(w, httpErr.StatusCode, body)
				return
			} else {
				msg := "InternalServerError"
				description := "Internal Server Error"
				body := &e{ErrorMessage: &msg, Description: &description}
				HttpWrite(w, 500, body)
				return
			}
		}
		if obj != nil {
			HttpWrite(w, http.StatusOK, obj)
		} else {
			HttpWrite(w, http.StatusOK, map[string]string{})
		}
	}).Methods(method)
}

func (b *BusinessLogic) AddRoutes(router *mux.Router) {
	b.addRoute(router, "", "GET", b.GetInstance)
}
