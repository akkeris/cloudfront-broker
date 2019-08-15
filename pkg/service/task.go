package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"cloudfront-broker/pkg/storage"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

const (
	actionCreateNew                   string = "create-new"
	actionCreateOrigin                string = "create-origin"
	actionCreateIAMUser               string = "create-iam-user"
	actionCreateAccessKey             string = "create-access-key"
	actionCreateOriginAccessIdentity  string = "create-origin-access-identity"
	actionIsOriginAccessIdentityReady string = "is-origin-access-identity-ready"
	actionCreateDistribution          string = "create-distribution"
	actionAddBucketPolicy             string = "add-bucket-policy"
	actionIsDistributionDeployed      string = "is-distribution-deployed"
	actionCreated                     string = "created"

	actionDeleteNew                  string = "delete-new"
	actionDisableDistribution        string = "disable-distribution"
	actionDeleteOrigin               string = "delete-origin"
	actionDeleteIAMUser              string = "delete-iam-user"
	actionIsDistributionDisabled     string = "is-distribution-disabled"
	actionDeleteDistribution         string = "delete-distribution"
	actionDeleteOriginAccessIdentity string = "delete-origin-access-identity"
	actionDeleted                    string = "deleted"

	actionDone string = "done"

	statusNew       string = "new"
	statusPending   string = "pending"
	statusDisabling string = "disabling"
	statusDeployed  string = "deployed"
	statusDeleted   string = "deleted"
	statusFailed    string = "failed"
	statusFinished  string = "finished"
)

// OriginID type cast to string
type OriginID struct {
	OriginID string
}

// IAMUser holds AWS IAM user
type IAMUser struct {
	OriginID string
	UserName string
}

var nextAction = map[string]string{
	actionCreateNew:                   actionCreateOrigin,
	actionCreateOrigin:                actionCreateIAMUser,
	actionCreateIAMUser:               actionCreateAccessKey,
	actionCreateAccessKey:             actionCreateOriginAccessIdentity,
	actionCreateOriginAccessIdentity:  actionIsOriginAccessIdentityReady,
	actionIsOriginAccessIdentityReady: actionCreateDistribution,
	actionCreateDistribution:          actionAddBucketPolicy,
	actionAddBucketPolicy:             actionIsDistributionDeployed,
	actionIsDistributionDeployed:      actionCreated,
	actionCreated:                     actionDone,

	actionDeleteNew:                  actionDisableDistribution,
	actionDisableDistribution:        actionDeleteIAMUser,
	actionDeleteIAMUser:              actionDeleteOrigin,
	actionDeleteOrigin:               actionIsDistributionDisabled,
	actionIsDistributionDisabled:     actionDeleteDistribution,
	actionDeleteDistribution:         actionDeleteOriginAccessIdentity,
	actionDeleteOriginAccessIdentity: actionDeleted,
	actionDeleted:                    actionDone,
}

func curTaskStop(curTask *storage.Task) *storage.Task {
	now := time.Now()
	curTask.FinishedAt = storage.SetNullTime(&now)
	return curTask
}

func curTaskFailed(curTask *storage.Task, msg string) *storage.Task {
	curTask.Status = statusFailed
	curTask.Result = storage.SetNullString(statusFailed)
	curTask.Metadata = storage.SetNullString(msg)
	return curTaskStop(curTask)
}

func curTaskFinished(curTask *storage.Task, result string, msg string) *storage.Task {
	curTask.Status = statusFinished
	curTask.Result = storage.SetNullString(result)
	curTask.Metadata = storage.SetNullString(msg)
	return curTaskStop(curTask)
}

func (svc *AwsConfig) getTaskState(distributionID string) (*osb.LastOperationResponse, error) {
	glog.Infof("===== getTaskState [%s] =====", distributionID)

	task, err := svc.stg.GetTaskByDistribution(distributionID)

	if err != nil {
		msg := fmt.Sprintf("getTaskState [%s]: error getting task: %s", distributionID, err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	taskState := &osb.LastOperationResponse{
		State:       osb.StateFailed,
		Description: aws.String("process failed"),
	}

	switch task.Status {
	case statusNew:
		fallthrough
	case statusPending:
		taskState.State = osb.StateInProgress
		taskState.Description = &task.Action
	case statusDeployed:
		fallthrough
	case statusDeleted:
		fallthrough
	case statusFinished:
		taskState.State = osb.StateSucceeded
		taskState.Description = &task.Result.String
	case statusFailed:
		fallthrough
	default:
		taskState.State = osb.StateFailed
		taskState.Description = &task.Result.String
	}

	return taskState, nil
}

// ActionCreateNew sets up the action to create a new distribution
func (svc *AwsConfig) ActionCreateNew(cf *cloudFrontInstance) error {
	glog.Infof("===== actionCreateNew [%s] =====", *cf.operationKey)

	err := svc.stg.NewDistribution(*cf.distributionID, *cf.planID, cf.billingCode, *cf.callerReference, statusPending)

	if err != nil {
		msg := fmt.Sprintf("actionCreateNew[%s]: error adding new distribution: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	now := time.Now()
	task := &storage.Task{
		DistributionID: *cf.distributionID,
		Action:         nextAction[actionCreateNew],
		Status:         statusNew,
		Retries:        0,
		OperationKey:   sql.NullString{String: *cf.operationKey, Valid: true},
		Result:         sql.NullString{String: OperationInProgress, Valid: true},
		Metadata:       sql.NullString{String: "", Valid: false},
		StartedAt:      storage.SetNullTime(&now),
	}

	task, err = svc.stg.AddTask(task)

	if err != nil {
		msg := fmt.Sprintf("actionCreateNew: error adding task: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (svc *AwsConfig) actionCreateOrigin(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreateOrigin [%s] =====", *cf.operationKey)

	if err := svc.createS3Bucket(cf); err != nil {
		msg := fmt.Sprintf("actionCreateOrigin[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error creating s3 bucket for origin")
		return curTask, errors.New(msg)
	}

	/*
	   encode originID into json to store as metadata
	*/
	originID := &OriginID{OriginID: *cf.s3Bucket.originID}
	originIDb, _ := json.Marshal(originID)
	curTask.Metadata = storage.SetNullString(string(originIDb))
	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionCreateIAMUser(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreateIAMUser [%s] =====", *cf.operationKey)

	originID := &OriginID{}
	_ = json.Unmarshal([]byte(curTask.Metadata.String), originID)
	s3BucketIn := svc.getBucket(originID.OriginID)

	if s3BucketIn != nil {
		if svc.isBucketReady(s3BucketIn) {
			cf.s3Bucket = s3BucketIn
			if err := svc.createIAMUser(cf); err != nil {
				msg := fmt.Sprintf("actionCreateIAMUser[%s]: error: %s", *cf.operationKey, err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, "error creating iam user")
				return curTask, errors.New(msg)
			}
		} else {
			curTask.Retries++
			return curTask, nil
		}
	}

	iAMUser := &IAMUser{
		OriginID: *cf.s3Bucket.originID,
		UserName: *cf.s3Bucket.iAMUser.userName,
	}

	iAMUserb, _ := json.Marshal(iAMUser)
	curTask.Metadata = sql.NullString{
		String: string(iAMUserb),
		Valid:  true,
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionCreateAccessKey(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreateAccessKey [%s] =====", *cf.operationKey)

	iAMUser := &IAMUser{}
	_ = json.Unmarshal([]byte(curTask.Metadata.String), iAMUser)

	s3BucketIn := svc.getBucket(iAMUser.OriginID)
	cf.s3Bucket = s3BucketIn

	if ok, err := svc.isIAMUserReady(iAMUser.UserName); ok {
		err = svc.createAccessKey(cf)
		if err != nil {
			msg := fmt.Sprintf("actionCreateAccessKey[%s]: error: %s", *cf.operationKey, err.Error())
			glog.Error(msg)
			curTask = curTaskFailed(curTask, "error creating access key")
			return curTask, errors.New(msg)
		}
		curTask.Result = storage.SetNullString("")
		curTask.Metadata = storage.SetNullString("")
	} else if err != nil {
		msg := fmt.Sprintf("actionCreateAccessKey[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error checking iam user")
		return curTask, errors.New(msg)
	} else {
		curTask.Retries++
		return curTask, nil
	}

	curTask.Action = nextAction[curTask.Action]
	curTask.Retries = 0
	return curTask, nil
}

func (svc *AwsConfig) actionCreateOriginAccessIdentity(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreateOriginAccessIdentity [%s] =====", *cf.operationKey)

	err := svc.createOriginAccessIdentity(cf)
	if err != nil {
		msg := fmt.Sprintf("actionCreateOriginAccessIdentity[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error creating origin access identity")
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionIsOriginAccessIdentityReady(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionIsOriginAccessIdentityReady [%s] =====", *cf.operationKey)

	ready, err := svc.isOriginAccessIdentityReady(cf)
	if err != nil {
		msg := fmt.Sprintf("actionIsOriginAccessIdentityReady [%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error creating origin access identity")
		return curTask, errors.New(msg)
	} else if !ready {
		curTask.Retries++
		glog.Infof("actionIsOriginAccessIdentityReady [%s]: retries: %3d", *cf.operationKey, curTask.Retries)
		return curTask, nil
	} else {
		curTask.Retries = 0
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionCreateDistribution(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreateDistribution [%s] =====", *cf.operationKey)

	err := svc.createDistribution(cf)
	if err != nil {
		msg := fmt.Sprintf("actionCreateDistribution[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error creating distribution")
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionAddBucketPolicy(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionAddBucketPolicy [%s] =====", *cf.operationKey)

	if err := svc.addBucketPolicy(cf); err != nil {
		msg := fmt.Sprintf("actionAddBucketPolicy[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error adding bucket policy")
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionIsDistributionDeployed(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionIsDistributionDeployed [%s] =====", *cf.operationKey)
	deployed, err := svc.isDistributionDeployed(cf)

	if err != nil {
		msg := fmt.Sprintf("actionIsDistributionDeployed [%s]: error checking distribution deployed: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	} else if !deployed {
		curTask.Retries++
		glog.Infof("actionIsDistributionDeployed [%s]: retries: %3d", *cf.operationKey, curTask.Retries)
		return curTask, nil
	} else {
		curTask.Retries = 0
	}

	curTask.Action = nextAction[curTask.Action]

	return curTask, nil
}

func (svc *AwsConfig) actionCreated(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreated [%s] =====", *cf.operationKey)

	err := svc.stg.UpdateDistributionStatus(*cf.distributionID, statusDeployed, false)
	if err != nil {
		msg := fmt.Sprintf("actionCreated: error updating distribution status: %s", err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	curTask = curTaskFinished(curTask, statusDeployed, "cloudfront distribution created and deployed")
	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

// ActionDeleteNew sets up the action to delete a distribution
func (svc *AwsConfig) ActionDeleteNew(cf *cloudFrontInstance) error {
	glog.Infof("===== actionDeleteNew [%s] =====", *cf.operationKey)

	err := svc.stg.UpdateDistributionStatus(*cf.distributionID, statusDisabling, false)
	if err != nil {
		msg := fmt.Sprintf("actionDeleteNew: error updating distribution status: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	now := time.Now()
	task := &storage.Task{
		DistributionID: *cf.distributionID,
		Action:         nextAction[actionDeleteNew],
		Status:         statusNew,
		Retries:        0,
		OperationKey:   storage.SetNullString(*cf.operationKey),
		Result:         storage.SetNullString(OperationInProgress),
		Metadata:       storage.SetNullString(""),
		StartedAt:      storage.SetNullTime(&now),
	}

	task, err = svc.stg.AddTask(task)

	if err != nil {
		msg := fmt.Sprintf("actionDeleteNew: error adding task: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (svc *AwsConfig) actionDisableDistribution(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionDisableDistribution [%s] =====", *cf.operationKey)

	_, err := svc.getCloudfrontDistribution(cf)

	if err != nil {
		msg := fmt.Sprintf("actionDisableDistribution [%s]: getting distribution from aws: %s", *cf.operationKey, err.Error())
		curTask = curTaskFailed(curTask, "cloudfront distribution error")
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	err = svc.disableCloudfrontDistribution(cf)
	if err != nil {
		msg := fmt.Sprintf("actionDisableDistribution [%s]: getting disabling distribution: %s", *cf.operationKey, err.Error())
		curTask = curTaskFailed(curTask, "error disabling distribution")
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionDeleteOrigin(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionDeleteOrigin [%s] =====", *cf.operationKey)

	err := svc.deleteS3Bucket(cf)
	if err != nil {
		msg := fmt.Sprintf("actionDeleteOrigin [%s]: deleting s3 bucket: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionDeleteIAMUser(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionDeleteIAMUser [%s] =====", *cf.operationKey)

	err := svc.deleteIAMUser(cf)
	if err != nil {
		msg := fmt.Sprintf("actionDeleteIAMUser [%s]: deleting iam user: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionIsDistributionDisabled(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionIsDistributionDisabled [%s] =====", *cf.operationKey)
	disabled, err := svc.isDistributionDisabled(cf)

	if err != nil {
		msg := fmt.Sprintf("actionIsDistributionDisabled[%s]: error checking distribution disabled: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFinished(curTask, statusFailed, msg)
		return curTask, errors.New(msg)
	} else if !disabled {
		curTask.Retries++
		glog.Infof("actionIsDistributionDisabled [%s]: retries: %3d", *cf.operationKey, curTask.Retries)
		return curTask, nil
	} else {
		curTask.Retries = 0
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionDeleteDistribution(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionDeleteDistribution [%s] =====", *cf.operationKey)

	err := svc.deleteDistribution(cf)
	if err != nil {
		msg := fmt.Sprintf("actionDeleteDistribution [%s]: deleting distribution: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionDeleteOriginAccessIdentity(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionDeleteOriginAccessIdentity [%s] =====", *cf.operationKey)

	err := svc.deleteOriginAccessIdentity(cf)
	if err != nil {
		msg := fmt.Sprintf("actionDeleteOriginAccessIdentity [%s]: deleting origin access identity: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) actionDeleted(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== actionCreated [%s] =====", *cf.operationKey)
	err := svc.stg.UpdateDistributionStatus(*cf.distributionID, statusDeleted, true)
	if err != nil {
		msg := fmt.Sprintf("actionCreated: error updating distribution status: %s", err.Error())
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	curTask = curTaskFinished(curTask, statusDeleted, "cloudfront distribution disabled and deleted")
	curTask.Action = nextAction[curTask.Action]
	return curTask, nil
}

var actions = map[string]func(*AwsConfig, *storage.Task, *cloudFrontInstance) (*storage.Task, error){
	actionCreateOrigin:                (*AwsConfig).actionCreateOrigin,
	actionCreateIAMUser:               (*AwsConfig).actionCreateIAMUser,
	actionCreateAccessKey:             (*AwsConfig).actionCreateAccessKey,
	actionCreateOriginAccessIdentity:  (*AwsConfig).actionCreateOriginAccessIdentity,
	actionIsOriginAccessIdentityReady: (*AwsConfig).actionIsOriginAccessIdentityReady,
	actionCreateDistribution:          (*AwsConfig).actionCreateDistribution,
	actionAddBucketPolicy:             (*AwsConfig).actionAddBucketPolicy,
	actionIsDistributionDeployed:      (*AwsConfig).actionIsDistributionDeployed,
	actionCreated:                     (*AwsConfig).actionCreated,
	actionDisableDistribution:         (*AwsConfig).actionDisableDistribution,
	actionDeleteIAMUser:               (*AwsConfig).actionDeleteIAMUser,
	actionDeleteOrigin:                (*AwsConfig).actionDeleteOrigin,
	actionIsDistributionDisabled:      (*AwsConfig).actionIsDistributionDisabled,
	actionDeleteDistribution:          (*AwsConfig).actionDeleteDistribution,
	actionDeleteOriginAccessIdentity:  (*AwsConfig).actionDeleteOriginAccessIdentity,
	actionDeleted:                     (*AwsConfig).actionDeleted,
}

// RunTasks is a go routine to run the actions in correct order.
// It will wait for AWS service to be available before going to next service
// Task status is in the tasks database table, so is safe to restart.
func (svc *AwsConfig) RunTasks() {
	var err error

	waitDur := time.Duration(time.Second * time.Duration(svc.waitSecs))

	glog.Info("===== RunTasks =====")
	for {
		var cf *cloudFrontInstance
		var curTask *storage.Task

		curTask, err = svc.stg.PopNextTask()

		if err != nil {
			if err == sql.ErrNoRows {
				glog.Error("RunTasks: no tasks")
				// <-ticker.C
				time.Sleep(waitDur)
				continue
			} else {
				msg := fmt.Sprintf("RunTask: error popping next task: %s", err.Error())
				glog.Error(msg)
				continue
			}
		}

		taskDur := time.Now().Sub(curTask.UpdatedAt)

		msg := fmt.Sprintf("RunTask: %s: %v < %v", curTask.TaskID, taskDur, waitDur)
		glog.Info(msg)

		if taskDur < waitDur {
			time.Sleep(time.Duration(time.Second))
			continue
		}

		cf, err = svc.getCloudfrontInstance(curTask.DistributionID)
		cf.operationKey = &curTask.OperationKey.String

		if action, ok := actions[curTask.Action]; ok {
			curTask.Status = statusPending
			curTask, err = action(svc, curTask, cf)

			if err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
		} else {
			msg := fmt.Sprintf("RunTasks[%s]: action %s not found", *cf.operationKey, curTask.Action)
			glog.Error(msg)
			curTask = curTaskFailed(curTask, msg)
		}
		if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
			msg := fmt.Sprintf("RunTask: error: %s", err.Error())
			glog.Error(msg)
		}
	}
}
