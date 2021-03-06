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

	"github.com/Masterminds/semver"
	"github.com/fatih/structs"
	"github.com/golang/glog"
	"github.com/nu7hatch/gouuid"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"cloudfront-broker/pkg/service"
	"cloudfront-broker/pkg/storage"
)

// BusinessLogic holds the data used for processing.
type BusinessLogic struct {
	sync.RWMutex

	storage *storage.PostgresStorage
	service *service.AwsConfig
}

var _ broker.Interface = &BusinessLogic{}

func newOpKey(prefix string) string {
	newUUID, _ := uuid.NewV4()
	return prefix + strings.Split(newUUID.String(), "-")[0]
}

// NewBusinessLogic creates and returns a BusinessLogic structure
func NewBusinessLogic(ctx context.Context, o Options) (*BusinessLogic, error) {
	dbStore, namePrefix, waitSecs, maxRetries, err := InitFromOptions(ctx, o)

	if err != nil {
		glog.Errorf("error initializing: %s", err.Error())
		return nil, errors.New("error initializing" + ": " + err.Error())
	}

	awsConfig, err := service.Init(dbStore, namePrefix, waitSecs, maxRetries)
	if err != nil {
		msg := fmt.Sprintf("error initializing the service: %s\n", err)
		glog.Fatalln(msg)
	}

	bl := &BusinessLogic{
		storage: dbStore,
		service: awsConfig,
	}

	return bl, nil
}

// InitFromOptions accepts parameters for runtime initilization
// It returns initialized values
func InitFromOptions(ctx context.Context, o Options) (*storage.PostgresStorage, string, int64, int64, error) {

	var err error
	namePrefix := o.NamePrefix
	waitSecs := o.WaitSecs
	maxRetries := o.MaxRetries

	glog.V(4).Infof("options: %#+v", o)

	if namePrefix == "" && os.Getenv("NAME_PREFIX") != "" {
		namePrefix = os.Getenv("NAME_PREFIX")
	}

	if namePrefix == "" {
		return nil, "", 0, 0, errors.New("the name prefix was not specified, set NAME_PREFIX in environment or provide it via the cli using -name-prefix")
	}

	if os.Getenv("WAIT_SECONDS") != "" {
		s, err := strconv.ParseInt(os.Getenv("WAIT_SECONDS"), 10, 64)
		if err != nil {
			return nil, "", 0, 0, errors.New("invalid value for WAIT_SECONDS, set WAIT_SECONDS in environment or provide via the cli using -wait-seconds")
		}
		waitSecs = s
	}
	glog.V(2).Infof("InitFromOptions: waitSecs: %d", waitSecs)

	if os.Getenv("MAX_RETRIES") != "" {
		s, err := strconv.ParseInt(os.Getenv("MAX_RETRIES"), 10, 64)
		if err != nil {
			return nil, "", 0, 0, errors.New("invalid value for MAX_RETRIES, set MAX_RETRIES in environment or provide via the cli using -max-retries")
		}
		maxRetries = s
	}
	glog.V(2).Infof("InitFromOptions: maxRetries: %d", maxRetries)

	stg, err := storage.InitStorage(ctx, o.DatabaseURL)
	return stg, namePrefix, waitSecs, maxRetries, err
}

// GetCatalog returns an  OSB catalog retrieved from the DB
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
	glog.V(4).Infof("catalog response: %#+v", osbResponse)

	response.CatalogResponse = *osbResponse

	return response, nil
}

// Provision starts the provisioning process
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

	if request.ServiceID == "" {
		return nil, UnprocessableEntityWithMessage("ServiceRequired", "The service ID was not provided.")
	}

	if request.PlanID == "" {
		return nil, UnprocessableEntityWithMessage("PlanRequired", "The plan ID was not provided.")
	}

	newUUID, _ := uuid.NewV4()
	callerReference := newUUID.String()

	operationKey := newOpKey("PRV")
	respOpKey := osb.OperationKey(operationKey)
	response.OperationKey = &respOpKey

	response.Async = true

	distributionID := request.InstanceID
	serviceID := request.ServiceID
	planID := request.PlanID

	ok, err := b.service.IsDuplicateInstance(distributionID)

	if err != nil {
		return nil, InternalServerErrWithMessage("error checking instance", err.Error())
	}

	if ok {
		return nil, ConflictErrorWithMessage("instance already provisioned, is provisioning or has been deleted")
	}

	err = b.service.CreateCloudFrontDistribution(distributionID, callerReference, operationKey, serviceID, planID, &request.OrganizationGUID)
	if err != nil {
		return nil, InternalServerErr()
	}

	return &response, nil
}

// Deprovision starts the de-provisioning process
func (b *BusinessLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.DeprovisionResponse{}

	if !request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncRequired", "The query parameter accepts_incomplete=true MUST be included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	distributionID := request.InstanceID

	deployed, err := b.service.IsDeployedInstance(distributionID)
	if err != nil {
		if err.Error() == "DistributionNotDeployed" {
			return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance found but not deployed")
		}
	}
	if !deployed {
		return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance not deployed")
	}

	operationKey := newOpKey("DPV")
	respOpKey := osb.OperationKey(operationKey)
	response.OperationKey = &respOpKey
	response.Async = true

	err = b.service.DeleteCloudFrontDistribution(distributionID, operationKey)
	if err != nil {
		return nil, InternalServerErr()
	}

	return &response, nil
}

// LastOperation return status of last operation for requested operation key
func (b *BusinessLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := &broker.LastOperationResponse{}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	glog.V(0).Infof("LastOperation: instance id: %s", request.InstanceID)

	distributionID := request.InstanceID

	found, err := b.service.IsDuplicateInstance(distributionID)

	if err != nil {
		return nil, BadRequestError(err.Error())
	}

	if !found {
		return nil, BadRequestError("instance not found")
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

// Bind gets and returns url, bucket and secret id/key
func (b *BusinessLogic) Bind(request *osb.BindRequest, c *broker.RequestContext) (*broker.BindResponse, error) {

	if request.AcceptsIncomplete {
		return nil, UnprocessableEntityWithMessage("AsyncNotSupported", "The query parameter accepts_incomplete=true MUST NOT included the request.")
	}

	if request.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	deployed, err := b.service.IsDeployedInstance(request.InstanceID)
	if err != nil {
		if err.Error() == "DistributionNotDeployed" {
			return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance found but not deployed")
		} else if err.Error() == "DistributionNotFound" {
			return nil, UnprocessableEntityWithMessage("InstanceNotFound", "instance not found")
		}
	}
	if !deployed {
		return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance not deployed")
	}

	cloudFrontInstance, err := b.service.GetCloudFrontInstanceSpec(request.InstanceID)

	if err != nil {
		return nil, InternalServerErrWithMessage("ErrGettingInstance", err.Error())
	}

	// TODO: Add binding(tag) to cloudfront distribution and S3 bucket, if not already in tag list

	response := &broker.BindResponse{
		BindResponse: osb.BindResponse{
			Async:       false,
			Credentials: structs.Map(cloudFrontInstance.Access),
		},
	}

	return response, nil
}

// Unbind is not used
func (b *BusinessLogic) Unbind(request *osb.UnbindRequest, c *broker.RequestContext) (*broker.UnbindResponse, error) {
	return nil, NotFoundWithMessage("BindingNotProvided", "Service un-binding is not provided")
}

// Update is not used
func (b *BusinessLogic) Update(request *osb.UpdateInstanceRequest, c *broker.RequestContext) (*broker.UpdateInstanceResponse, error) {
	// Your logic for updating a service goes here.
	response := broker.UpdateInstanceResponse{}
	if request.AcceptsIncomplete {
		response.Async = request.AcceptsIncomplete
	}

	return &response, nil
}

// GetInstance returns information about an instance

func (b *BusinessLogic) FetchInstance(r *GetInstanceRequest, c *broker.RequestContext) (*GetInstanceResponse, error) {
	instanceID := r.InstanceID

	if instanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	deployed, err := b.service.IsDeployedInstance(instanceID)
	if err != nil {
		if err.Error() == "DistributionNotDeployed" {
			return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance found but not deployed")
		} else if err.Error() == "DistributionNotFound" {
			return nil, UnprocessableEntityWithMessage("InstanceNotFound", "instance not found")
		}
	}
	if !deployed {
		return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance not deployed")
	}

	cloudFrontInstance, err := b.service.GetCloudFrontInstanceSpec(instanceID)

	if err != nil {
		return nil, InternalServerErrWithMessage("ErrGettingInstance", err.Error())
	}

	resp := &GetInstanceResponse{
		ServiceID:    cloudFrontInstance.ServiceID,
		PlanID:       cloudFrontInstance.PlanID,
		DashboardURL: nil,
		Parameters:   structs.Map(cloudFrontInstance),
	}

	return resp, nil
}

// GetBinding validates binding_id is in cf tags then returns credentials, see Bind()
// func (b *BusinessLogic) GetBinding(instanceID string, vars map[string]string, context *broker.RequestContext) (interface{}, error) {
func (b *BusinessLogic) FetchBinding(r *osb.GetBindingRequest, c *broker.RequestContext) (*osb.GetBindingResponse, error) {
	if r.InstanceID == "" {
		return nil, UnprocessableEntityWithMessage("InstanceRequired", "The instance ID was not provided.")
	}

	deployed, err := b.service.IsDeployedInstance(r.InstanceID)
	if err != nil {
		if err.Error() == "DistributionNotDeployed" {
			return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance found but not deployed")
		} else if err.Error() == "DistributionNotFound" {
			return nil, UnprocessableEntityWithMessage("InstanceNotFound", "instance not found")
		}
	}
	if !deployed {
		return nil, UnprocessableEntityWithMessage("InstanceNotDeployed", "instance not deployed")
	}

	// TODO: check if binding id is in tags for cloudfront distribution

	cloudFrontInstance, err := b.service.GetCloudFrontInstanceSpec(r.InstanceID)

	if err != nil {
		return nil, InternalServerErrWithMessage("ErrGettingInstance", err.Error())
	}

	res := &osb.GetBindingResponse{
		Credentials: structs.Map(cloudFrontInstance.Access),
	}

	return res, nil

}

// ValidateBrokerAPIVersion verifies the client OSB version with support OSB versions
func (b *BusinessLogic) ValidateBrokerAPIVersion(version string) error {
	c, err := semver.NewConstraint(">=" + OSBVersion)
	if err != nil {
		return errors.New("invalid internal version")
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		msg := fmt.Sprintf("invalid received version: %s", version)
		return errors.New(msg)
	}

	a := c.Check(v)

	if !a {
		msg := fmt.Sprintf("unsupported version: %s < %s", version, OSBVersion)
		return errors.New(msg)
	}

	return nil
}

// RunTasksInBackground starts the background processing
func (b *BusinessLogic) RunTasksInBackground(ctx context.Context) error {
	b.service.RunTasks()
	// This should never return
	return errors.New("system error")
}
