package broker

import (
  "context"
  "errors"
  "net/http"
  "os"
  "sync"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/golang/glog"
  "github.com/nu7hatch/gouuid"
  "k8s.io/klog"

  osb "github.com/pmorie/go-open-service-broker-client/v2"
  "github.com/pmorie/osb-broker-lib/pkg/broker"

  "cloudfront-broker/pkg/service"
  "cloudfront-broker/pkg/storage"
)

// BusinessLogic provides an implementation of the broker.BusinessLogic
// interface.
type BusinessLogic struct {
  // Indicates if the broker should handle the requests asynchronously.
  async bool
  // Synchronize go routines.
  sync.RWMutex
  // Add fields here! These fields are provided purely as an example
  dbStore *storage.PostgresStorage
  service *service.AwsConfigSpec
  namePrefix  string
  port  string

  instances map[string]*storage.InstanceSpec
}


// NewBusinessLogic is a hook that is called with the Options the program is run
// with. NewBusinessLogic is the place where you will initialize your
// BusinessLogic the parameters passed in.
func NewBusinessLogic(ctx context.Context, o Options) (*BusinessLogic, error) {
	// For example, if your BusinessLogic requires a parameter from the command
	// line, you would unpack it from the Options and set it on the
	// BusinessLogic here.

  dbStore, namePrefix, err := InitFromOptions(ctx, o)

  if err != nil {
    klog.Errorf("error initializing: %s", err.Error())
    return nil, errors.New("error initializing" + ": " + err.Error())
  }

  awsConfig, err := service.Init(namePrefix)

  klog.Infof("namePrefix=%s", namePrefix)

  return &BusinessLogic{
		async:      o.Async,
		dbStore:    dbStore,
		service:    awsConfig,
		namePrefix: namePrefix,
		instances:  make(map[string]*storage.InstanceSpec, 10),
	}, nil
}

func InitFromOptions(ctx context.Context, o Options) (*storage.PostgresStorage, string, error) {
  klog.Infof("options: %#+v", o)
  if o.NamePrefix == "" && os.Getenv("NAME_PREFIX") != "" {
    o.NamePrefix = os.Getenv("NAME_PREFIX")
  }
  if o.NamePrefix == "" {
    return nil, "", errors.New("the name prefix was not specified, set NAME_PREFIX in your environment or provide it via the cli using -name-prefix")
  }

  stg, err := storage.InitStorage(ctx, o.DatabaseUrl)
  return stg, o.NamePrefix, err
}

var _ broker.Interface = &BusinessLogic{}

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
  var ctx aws.Context
	b.Lock()
	defer b.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response := broker.ProvisionResponse{}

	if !request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncRequired", "The query parameter accepts_incomplete=true MUST be included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	newInstance := &storage.InstanceSpec{
		ID:        request.InstanceID,
		ServiceID: request.ServiceID,
		PlanID:    request.PlanID,
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

	newUuid, _ := uuid.NewV4()
	callerReference := newUuid.String()

	err := b.service.CreateCloudFrontDistribution(ctx, callerReference, newInstance.BillingCode, newInstance.PlanID)

	if err != nil {
	  return nil, UnprocessableEntityWithMessage(err.Error(), "problem creating distribution")
  }

	return &response, nil
}

func (b *BusinessLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {
	// Your deprovision business logic goes here

	// example implementation:
	b.Lock()
	defer b.Unlock()

	response := broker.DeprovisionResponse{}

	delete(b.instances, request.InstanceID)

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	return &response, nil
}

func (b *BusinessLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	// Your last-operation business logic goes here

	return nil, nil
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

