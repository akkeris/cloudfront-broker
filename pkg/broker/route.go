package broker

import (
	"encoding/json"
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
	w.WriteHeader(status)
	w.Write(data)
}

func (b *BusinessLogic) addOSBFetchInstance(router *mux.Router) {
	router.HandleFunc("/v2/service_instances/{instance_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		req := GetInstanceRequest{
			InstanceID: vars["instance_id"],
		}

		c := &broker.RequestContext{
			Writer:  w,
			Request: r,
		}

		glog.V(4).Infof("Received FetchInstanceRequest for instanceID %q", req.InstanceID)

		resp, err := b.FetchInstance(&req, c)

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

func (b *BusinessLogic) addOSBFetchBindingRoute(router *mux.Router) {
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

		glog.V(4).Infof("Received FetchBindRequest for instanceID %q, bindingID %q", req.InstanceID, req.BindingID)

		resp, err := b.FetchBinding(&req, c)

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
	b.addOSBFetchInstance(router)
	b.addOSBFetchBindingRoute(router)
}
