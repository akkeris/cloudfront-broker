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

// httpWrite is used to write data to outgoing http response
func httpWrite(w http.ResponseWriter, status int, obj interface{}) {
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
	glog.V(2).Infof("AddRoute: Adding route %s", path)
	router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		c := broker.RequestContext{Request: r, Writer: w}
		obj, herr := handler(vars["instance_id"], vars, &c)
		if herr != nil {
			if httpErr, ok := osb.IsHTTPError(herr); ok {
				body := &errorSpec{}
				if httpErr.Description != nil {
					body.Description = httpErr.Description
				}
				if httpErr.ErrorMessage != nil {
					body.ErrorMessage = httpErr.ErrorMessage
				}
				httpWrite(w, httpErr.StatusCode, body)
				return
			}
			msg := "InternalServerError"
			description := "Internal Server Error"
			body := &errorSpec{ErrorMessage: &msg, Description: &description}
			httpWrite(w, 500, body)
			return
		}
		if obj != nil {
			httpWrite(w, http.StatusOK, obj)
		} else {
			httpWrite(w, http.StatusOK, map[string]string{})
		}
	}).Methods(method)
}

func (b *BusinessLogic) OSBAddGetInstance(router *mux.Router) {
	router.HandleFunc("/v2/service_instances/{instance_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		req := GetInstanceRequest{
			InstanceID: vars["instance_id"],
		}

		c := &broker.RequestContext{
			Writer:  w,
			Request: r,
		}

		resp, err := b.GetInstance(&req, c)

		if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				body := &errorSpec{
					Description:  httpErr.Description,
					ErrorMessage: httpErr.ErrorMessage,
				}
				httpWrite(w, httpErr.StatusCode, body)
			} else {
				httpWrite(w, http.StatusInternalServerError, InternalServerErr())
			}
			return
		}
		httpWrite(w, http.StatusOK, resp)
	}).Methods("GET")
}

func (b *BusinessLogic) OSBAddGetBindingRoute(router *mux.Router) {
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		req := osb.GetBindingRequest{
			InstanceID: vars["instance_id"],
			BindingID:  vars["binding_id"],
		}

		c := &broker.RequestContext{
			Writer:  w,
			Request: r,
		}

		resp, err := b.GetBinding(&req, c)

		if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				body := &errorSpec{
					Description:  httpErr.Description,
					ErrorMessage: httpErr.ErrorMessage,
				}
				httpWrite(w, httpErr.StatusCode, body)
			} else {
				httpWrite(w, http.StatusInternalServerError, InternalServerErr())
			}
			return
		}
		httpWrite(w, http.StatusOK, resp)
	}).Methods("GET")
}

// AddRoutes adds extra routes not in broker interface
func (b *BusinessLogic) AddRoutes(router *mux.Router) {
	//	b.addRoute(router, "", "GET", b.GetInstance)
	// 	b.addRoute(router, "/service_bindings/{binding_id}", "GET", b.GetBinding)
	b.OSBAddGetInstance(router)
	b.OSBAddGetBindingRoute(router)
}
