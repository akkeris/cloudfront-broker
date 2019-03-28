package broker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/nu7hatch/gouuid"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"cloudfront-broker/pkg/service"
	"cloudfront-broker/pkg/storage"
)

type BusinessLogic struct {
	async bool
	sync.RWMutex

	storage *storage.PostgresStorage
	service *service.AwsConfig
	port    string
}

var _ broker.Interface = &BusinessLogic{}

func newOpKey(prefix string) string {
	newUuid, _ := uuid.NewV4()
	return prefix + strings.Split(newUuid.String(), "-")[0]
}

func NewBusinessLogic(ctx context.Context, o Options) (*BusinessLogic, error) {
	dbStore, namePrefix, waitSecs, err := InitFromOptions(ctx, o)

	if err != nil {
		glog.Errorf("error initializing: %s", err.Error())
		return nil, errors.New("error initializing" + ": " + err.Error())
	}

	awsConfig, err := service.Init(dbStore, namePrefix, waitSecs)
	if err != nil {
		msg := fmt.Sprintf("error initializing the service: %s\n", err)
		glog.Fatalln(msg)
	}

	glog.Infof("namePrefix=%s", namePrefix)

	bl := &BusinessLogic{
		async:   o.Async,
		storage: dbStore,
		service: awsConfig,
	}

	return bl, nil
}

func InitFromOptions(ctx context.Context, o Options) (*storage.PostgresStorage, string, int64, error) {

	var err error
	namePrefix := o.NamePrefix
	waitSecs := o.WaitSecs

	glog.Infof("options: %#+v", o)

	if namePrefix == "" && os.Getenv("NAME_PREFIX") != "" {
		namePrefix = os.Getenv("NAME_PREFIX")
	}

	if namePrefix == "" {
		return nil, "", 0, errors.New("the name prefix was not specified, set NAME_PREFIX in your environment or provide it via the cli using -name-prefix")
	}

	if os.Getenv("WAIT_SECONDS") != "" {
		s, err := strconv.ParseInt(os.Getenv("WAIT_SECONDS"), 10, 64)
		if err != nil {
			return nil, "", 0, errors.New("Invalid value for WAIT_SECONDS")
		}
		waitSecs = s
	}

	stg, err := storage.InitStorage(ctx, o.DatabaseUrl)
	return stg, namePrefix, waitSecs, err
}

func (b *BusinessLogic) GetCatalog(c *broker.RequestContext) (*broker.CatalogResponse, error) {
	var err error

	response := &broker.CatalogResponse{}
	osbResponse := &osb.CatalogResponse{}

	osbResponse.Services, err = b.storage.GetServicesCatalog()
	if err != nil {
		description := "Error getting catalog"
		glog.Errorf("%s: %s", description, err.Error())
		return nil, osb.HTTPStatusCodeError{
			StatusCode:  http.StatusInternalServerError,
			Description: &description,
		}
	}
	glog.Infof("catalog response: %#+v", osbResponse)

	response.CatalogResponse = *osbResponse

	return response, nil
}

func (b *BusinessLogic) Provision(request *osb.ProvisionRequest, c *broker.RequestContext) (*broker.ProvisionResponse, error) {
	var billingCode string

	b.Lock()
	defer b.Unlock()

	response := broker.ProvisionResponse{}

	if !request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncRequired", "The query parameter accepts_incomplete=true MUST be included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	if request.ServiceID == "" {
		return nil, UnprocessableEntityWithMessage("ServiceRequired", "The service ID was not provided.")
	}

	if request.PlanID == "" {
		return nil, UnprocessableEntityWithMessage("PlanRequired", "The plan ID was not provided.")
	}

	newUuid, _ := uuid.NewV4()
	callerReference := newUuid.String()

	operationKey := newOpKey("PRV")
	respOpKey := osb.OperationKey(operationKey)
	response.OperationKey = &respOpKey

	distributionID := request.InstanceID
	serviceID := request.ServiceID
	planID := request.PlanID

	for k, v := range request.Parameters {
		switch vv := v.(type) {
		case string:
			if k == "billing_code" {
				billingCode = vv
				glog.Info(k, " is string ", vv)
			}
		}
	}

	if billingCode == "" {
		return nil, UnprocessableEntityWithMessage("BillingCodeRequired", "The billing code was not provided.")
	}

	ok, err := b.service.IsDuplicateInstance(distributionID)

	if err != nil {
		return nil, InternalServerErrWithMessage("error checking instance", err.Error())
	}

	if ok {
		return nil, ConflictErrorWithMessage("instance already provisioned, is provisioning or has been deleted")
	}

	err = b.service.CreateCloudFrontDistribution(distributionID, callerReference, operationKey, serviceID, planID, billingCode)
	if err != nil {
		return nil, InternalServerErr()
	}

	return &response, nil
}

func (b *BusinessLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.DeprovisionResponse{}

	// newUuid, _ := uuid.NewV4()
	// callerReference := newUuid.String()

	if !request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncRequired", "The query parameter accepts_incomplete=true MUST be included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	distributionID := request.InstanceID

	if deployed, err := b.service.IsDeployedInstance(distributionID); err != nil {
		if err.Error() == "DistributionNotDeployed" {
			return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance found but not deployed")
		}
	}

	operationKey := newOpKey("DPV")
	respOpKey := osb.OperationKey(operationKey)
	response.OperationKey = &respOpKey
	response.Async = b.async

	err := b.service.DeleteCloudFrontDistribution(distributionID, operationKey)
	if err != nil {
		return nil, InternalServerErr()
	}

	return &response, nil
}

func (b *BusinessLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := &broker.LastOperationResponse{}

	glog.Infof("request: %+#v", request)

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	glog.Infof("LastOperation: instance id: %s", request.InstanceID)

	/*
	  if request.ServiceID != nil {
	    glog.Infof("lastop: service id: %s", *request.ServiceID)
	  }
	  if request.PlanID != nil {
	    glog.Infof("lastop: plan id: %s", *request.PlanID)
	  }
	  if request.OperationKey == nil {
	    return nil, UnprocessableEntityWithMessage("OperationKeyRequired", "The operation key was not provided.")
	  }
	  operationKey := string(*request.OperationKey)
	*/

	distributionID := request.InstanceID

	found, err := b.service.IsDuplicateInstance(distributionID)

	if err != nil {
		return nil, BadRequestError(err.Error())
	} else {
		if !found {
			return nil, BadRequestError("instance not found")
		}
	}

	state, err := b.service.CheckLastOperation(distributionID)

	if err != nil {
		return nil, InternalServerErr()
	}

	response.State = state.State
	response.Description = state.Description
	response.LastOperationResponse = *state

	return response, nil
}

func (b *BusinessLogic) Bind(request *osb.BindRequest, c *broker.RequestContext) (*broker.BindResponse, error) {
	// Your bind business logic goes here

	// example implementation:
	var response broker.BindResponse

	/*
	   b.Lock()
	   defer b.Unlock()

	   instance, ok := b.instances[request.InstanceID]
	   if !ok {
	     return nil, osb.HTTPStatusCodeError{
	       StatusCode: http.StatusNotFound,
	     }
	   }

	   response := broker.BindResponse{
	     BindResponse: osb.BindResponse{
	       Credentials: instance.Params,
	     },
	   }
	   if request.AcceptsIncomplete {
	     response.Async = b.async
	   }
	*/

	return &response, nil
}

func (b *BusinessLogic) Unbind(request *osb.UnbindRequest, c *broker.RequestContext) (*broker.UnbindResponse, error) {
	// Your unbind business logic goes here
	return &broker.UnbindResponse{}, nil
}

func (b *BusinessLogic) Update(request *osb.UpdateInstanceRequest, c *broker.RequestContext) (*broker.UpdateInstanceResponse, error) {
	// Your logic for updating a service goes here.
	response := broker.UpdateInstanceResponse{}
	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	return &response, nil
}

func (b *BusinessLogic) ValidateBrokerAPIVersion(version string) error {
	return nil
}

func (b *BusinessLogic) RunTasksInBackground(ctx context.Context) {
	b.service.RunTasks()
}
