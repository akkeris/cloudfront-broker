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
	"time"

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

	dbStore    *storage.PostgresStorage
	service    *service.AwsConfigSpec
	namePrefix string
	port       string

	instances map[string]*storage.InstanceSpec
}

var _ broker.Interface = &BusinessLogic{}

func newOpKey(prefix string) string {
	newUuid, _ := uuid.NewV4()
	return prefix + strings.Split(newUuid.String(), "-")[0]
}

func NewBusinessLogic(ctx context.Context, o Options) (*BusinessLogic, error) {
	dbStore, namePrefix, waitCnt, waitSecs, err := InitFromOptions(ctx, o)

	if err != nil {
		glog.Errorf("error initializing: %s", err.Error())
		return nil, errors.New("error initializing" + ": " + err.Error())
	}

	awsConfig, err := service.Init(namePrefix, waitCnt, waitSecs)
	if err != nil {
		msg := fmt.Sprintf("error initializing the service: %s\n", err)
		glog.Fatalln(msg)
	}

	glog.Infof("namePrefix=%s", namePrefix)

	return &BusinessLogic{
		async:      o.Async,
		dbStore:    dbStore,
		service:    awsConfig,
		namePrefix: namePrefix,

		instances: make(map[string]*storage.InstanceSpec, 10),
	}, nil
}

func InitFromOptions(ctx context.Context, o Options) (*storage.PostgresStorage, string, int, time.Duration, error) {

	var err error
	namePrefix := o.NamePrefix
	var waitCnt int = o.WaitCnt
	var waitSecs time.Duration = time.Duration(o.WaitSecs)

	glog.Infof("options: %#+v", o)

	if namePrefix == "" && os.Getenv("NAME_PREFIX") != "" {
		namePrefix = os.Getenv("NAME_PREFIX")
	}

	if namePrefix == "" {
		return nil, "", 0, 0, errors.New("the name prefix was not specified, set NAME_PREFIX in your environment or provide it via the cli using -name-prefix")
	}

	if os.Getenv("WAIT_COUNT") != "" {
		c, err := strconv.Atoi(os.Getenv("WAIT_COUNT"))
		if err != nil {
			return nil, "", 0, 0, errors.New("Invalid value for WAIT_COUNT")
		}
		waitCnt = c
	}

	if os.Getenv("WAIT_SECONDS") != "" {
		s, err := strconv.Atoi(os.Getenv("WAIT_SECONDS"))
		if err != nil {
			return nil, "", 0, 0, errors.New("Invalid value for WAIT_SECONDS")
		}
		waitSecs = time.Duration(s)
	}

	stg, err := storage.InitStorage(ctx, o.DatabaseUrl)
	return stg, namePrefix, waitCnt, waitSecs, err
}

func (b *BusinessLogic) GetCatalog(c *broker.RequestContext) (*broker.CatalogResponse, error) {
	var err error

	response := &broker.CatalogResponse{}
	osbResponse := &osb.CatalogResponse{}
	osbResponse.Services, err = b.dbStore.GetServices()
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
	b.Lock()
	defer b.Unlock()

	response := broker.ProvisionResponse{}

	if !request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncRequired", "The query parameter accepts_incomplete=true MUST be included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	newUuid, _ := uuid.NewV4()
	callerReference := newUuid.String()

	operationKey := newOpKey("PRV")
	respOpKey := osb.OperationKey(operationKey)
	response.OperationKey = &respOpKey

	newInstance := &storage.InstanceSpec{
		ID:           request.InstanceID,
		ServiceId:    request.ServiceID,
		PlanId:       request.PlanID,
		OperationKey: operationKey,
	}

	// Check to see if this is the same instance
	if i := b.instances[request.InstanceID]; i != nil {
		if i.Match(newInstance) {
			response.Exists = true
			return &response, nil
		} else {
			// Instance ID in use, this is a conflict.
			description := "InstanceID in use"
			return nil, ConflictErrorWithMessage(description)
		}
	}
	b.instances[request.InstanceID] = newInstance

	err := b.service.CreateCloudFrontDistribution(callerReference, newInstance)
	if err != nil {
		return nil, InternalServerErr()
	}

	return &response, nil
}

func (b *BusinessLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.DeprovisionResponse{}

	newUuid, _ := uuid.NewV4()
	callerReference := newUuid.String()

	if !request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncRequired", "The query parameter accepts_incomplete=true MUST be included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	operationKey := newOpKey("DPV")
	respOpKey := osb.OperationKey(operationKey)
	response.OperationKey = &respOpKey
	response.Async = b.async

	oldInstance := &storage.InstanceSpec{
		ID:           request.InstanceID,
		ServiceId:    request.ServiceID,
		PlanId:       request.PlanID,
		OperationKey: operationKey,
	}

	err := b.service.DeleteCloudFrontDistribution(callerReference, oldInstance)
	if err != nil {
		return nil, InternalServerErr()
	}

	delete(b.instances, request.InstanceID)

	return &response, nil
}

func (b *BusinessLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.LastOperationResponse{}

	/*
	  if *request.OperationKey == "" {
	    return nil, UnprocessableEntityWithMessage("OperationKeyRequired", "The operation key was not provided.")
	  }
	*/

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	// opKey := string(*request.OperationKey)

	instance := &storage.InstanceSpec{
		ID: request.InstanceID,
	}

	status, err := b.service.CheckLastOperation(instance)

	if err != nil {
		msg := fmt.Sprintf("error checking last operation for %s: %s", instance.ID, err)
		glog.Error(msg)
		return nil, InternalServerErrWithMessage("InternalServerError", "server error when checking status of last operation")
	}

	response.State = osb.LastOperationState(*status.Status)
	response.Description = status.Description

	return &response, nil
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
