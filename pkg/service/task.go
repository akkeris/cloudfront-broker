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
	ActionCreateNew                  string = "create-new"
	ActionCreateOrigin               string = "create-origin"
	ActionCreateIAMUser              string = "create-iam-user"
	ActionCreateAccessKey            string = "create-access-key"
	ActionCreateOriginAccessIdentity string = "create-origin-access-identity"
	ActionCreateDistribution         string = "create-distribution"
	ActionAddBucketPolicy            string = "add-bucket-policy"
	ActionIsDistributionDeployed     string = "is-distribution-deployed"

	ActionDeleteNew                  string = "delete-new"
	ActionDisableDistribution        string = "disable-distribution"
	ActionDeleteOrigin               string = "delete-origin"
	ActionDeleteIAMUser              string = "delete-iam-user"
	ActionDeleteDistribution         string = "delete-distribution"
	ActionDeleteOriginAccessIdentity string = "delete-origin-access-identity"
	ActionDone                       string = "done"

	StatusNew       string = "new"
	StatusPending   string = "pending"
	StatusDisabling string = "disabling"
	StatusDeployed  string = "deployed"
	StatusDeleted   string = "deleted"
	StatusFailed    string = "failed"
	StatusFinished  string = "finished"
)

var NextAction = map[string]string{
	ActionCreateNew:                  ActionCreateOrigin,
	ActionCreateOrigin:               ActionCreateIAMUser,
	ActionCreateIAMUser:              ActionCreateAccessKey,
	ActionCreateAccessKey:            ActionCreateOriginAccessIdentity,
	ActionCreateOriginAccessIdentity: ActionCreateDistribution,
	ActionCreateDistribution:         ActionAddBucketPolicy,
	ActionAddBucketPolicy:            ActionIsDistributionDeployed,
	ActionIsDistributionDeployed:     ActionDone,

	ActionDeleteNew:                  ActionDisableDistribution,
	ActionDisableDistribution:        ActionDeleteOrigin,
	ActionDeleteOrigin:               ActionDeleteIAMUser,
	ActionDeleteIAMUser:              ActionDeleteDistribution,
	ActionDeleteDistribution:         ActionDeleteOriginAccessIdentity,
	ActionDeleteOriginAccessIdentity: ActionDone,
}

type OriginID struct {
	OriginID string
}

type IAMUser struct {
	OriginID string
	UserName string
}

func curTaskStop(curTask *storage.Task) *storage.Task {
	now := time.Now()
	curTask.FinishedAt = storage.SetNullTime(&now)
	return curTask
}

func curTaskFailed(curTask *storage.Task, msg string) *storage.Task {
	curTask.Status = StatusFailed
	curTask.Result = storage.SetNullString(storage.StatusFailed)
	curTask.Metadata = storage.SetNullString(msg)
	return curTaskStop(curTask)
}

func curTaskFinished(curTask *storage.Task, result string, msg string) *storage.Task {
	curTask.Status = StatusFinished
	curTask.Result = storage.SetNullString(result)
	curTask.Metadata = storage.SetNullString(msg)
	return curTaskStop(curTask)
}

func setTask(task *storage.Task, action string, status string, result *string, metadata *string, finished bool) *storage.Task {
	task.Action = action
	task.Status = status
	if result != nil {
		resultB, _ := json.Marshal(result)
		task.Result.String = string(resultB)
		task.Result.Valid = true
	} else {
		task.Result.String = ""
		task.Result.Valid = false
	}

	if metadata != nil {
		task.Metadata = sql.NullString{
			String: *metadata,
			Valid:  true,
		}
	} else {
		task.Metadata = sql.NullString{
			String: "",
			Valid:  false,
		}
	}

	if finished {
		task.FinishedAt.Time = time.Now()
		task.FinishedAt.Valid = true
	} else {
		task.FinishedAt.Valid = false
	}

	return task
}

func (svc *AwsConfig) GetTaskState(distributionID string) (*osb.LastOperationResponse, error) {
	glog.Infof("===== GetTaskState [%s] =====", distributionID)

	task, err := svc.stg.GetTaskByDistribution(distributionID)

	if err != nil {
		msg := fmt.Sprintf("GetTaskState [%s]: error getting task: %s", distributionID, err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	taskState := &osb.LastOperationResponse{
		State:       osb.StateFailed,
		Description: aws.String("process failed"),
	}

	switch task.Status {
	case StatusNew:
		fallthrough
	case StatusPending:
		taskState.State = osb.StateInProgress
		taskState.Description = &task.Action
	case StatusDeployed:
		fallthrough
	case StatusDeleted:
		fallthrough
	case StatusFinished:
		taskState.State = osb.StateSucceeded
		taskState.Description = &task.Action
	default:
		taskState.State = osb.StateFailed
		taskState.Description = &task.Result.String
	}

	return taskState, nil
}

func (svc *AwsConfig) ActionCreateNew(cf *cloudFrontInstance) error {
	glog.Infof("===== ActionCreateNew [%s] =====", *cf.operationKey)

	err := svc.stg.NewDistribution(*cf.distributionID, *cf.planID, *cf.billingCode, *cf.callerReference, StatusPending)

	if err != nil {
		msg := fmt.Sprintf("ActionCreateNew[%s]: error adding new distribution: %s", cf.operationKey, err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	task := &storage.Task{
		DistributionID: *cf.distributionID,
		Action:         NextAction[ActionCreateNew],
		Status:         StatusNew,
		Retries:        0,
		OperationKey:   sql.NullString{String: *cf.operationKey, Valid: true},
		Result:         sql.NullString{String: OperationInProgress, Valid: true},
		Metadata:       sql.NullString{String: "", Valid: false},
	}

	task, err = svc.stg.AddTask(task)

	if err != nil {
		msg := fmt.Sprintf("ActionCreateNew: error adding task: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (svc *AwsConfig) ActionCreateOrigin(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== ActionCreateOrigin [%s] =====", *cf.operationKey)

	if err := svc.createS3Bucket(cf); err != nil {
		msg := fmt.Sprintf("ActionCreateOrigin[%s]: error: %s", *cf.operationKey, err.Error())
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
	curTask.Action = NextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) ActionCreateIAMUser(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== ActionCreateIAMUser [%s] =====", *cf.operationKey)

	originID := &OriginID{}
	_ = json.Unmarshal([]byte(curTask.Metadata.String), originID)
	s3BucketIn := svc.getBucket(originID.OriginID)

	if s3BucketIn != nil {
		if svc.isBucketReady(s3BucketIn) {
			cf.s3Bucket = s3BucketIn
			if err := svc.createIAMUser(cf); err != nil {
				msg := fmt.Sprintf("ActionCreateIAMUser[%s]: error: %s", *cf.operationKey, err.Error())
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

	curTask.Action = NextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) ActionCreateAccessKey(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== ActionCreateAccessKey [%s] =====", *cf.operationKey)

	iAMUser := &IAMUser{}
	_ = json.Unmarshal([]byte(curTask.Metadata.String), iAMUser)

	s3BucketIn := svc.getBucket(iAMUser.OriginID)
	cf.s3Bucket = s3BucketIn

	if ok, err := svc.isIAMUserReady(iAMUser.UserName); ok {
		err = svc.createAccessKey(cf)
		if err != nil {
			msg := fmt.Sprintf("ActionCreateAccessKey[%s]: error: %s", *cf.operationKey, err.Error())
			glog.Error(msg)
			curTask = curTaskFailed(curTask, "error creating access key")
			return curTask, errors.New(msg)
		} else {
			curTask.Result = storage.SetNullString("")
			curTask.Metadata = storage.SetNullString("")
		}
	} else if err != nil {
		msg := fmt.Sprintf("ActionCreateAccessKey[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error checking iam user")
		return curTask, errors.New(msg)
	} else {
		curTask.Retries++
		return curTask, nil
	}

	curTask.Action = NextAction[curTask.Action]
	curTask.Retries = 0
	return curTask, nil
}

func (svc *AwsConfig) ActionCreateOriginAccessIdentity(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== ActionCreateOriginAccessIdentity [%s] =====", *cf.operationKey)

	err := svc.createOriginAccessIdentity(cf)
	if err != nil {
		msg := fmt.Sprintf("ActionCreateOriginAccessIdentity[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error creating origin access identity")
		return curTask, errors.New(msg)
	}

	curTask.Action = NextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) ActionCreateDistribution(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== ActionCreateDistribution [%s] =====", *cf.operationKey)

	err := svc.createDistribution(cf)
	if err != nil {
		msg := fmt.Sprintf("ActionCreateDistribution[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error creating distribution")
		return curTask, errors.New(msg)
	}

	curTask.Action = NextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) ActionAddBucketPolicy(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Infof("===== ActionAddBucketPolicy [%s] =====", *cf.operationKey)

	if err := svc.addBucketPolicy(cf); err != nil {
		msg := fmt.Sprintf("ActionAddBucketPolicy[%s]: error: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFailed(curTask, "error adding bucket policy")
		return curTask, errors.New(msg)
	}

	curTask.Action = NextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) ActionIsDistributionDeployed(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Info("===== ActionIsDistributionDeployed [%s] =====", *cf.operationKey)
	distOut, err := svc.getCloudfrontDistribution(cf)

	if err != nil {
		msg := fmt.Sprintf("ActionEnableDistribution[%s]: error getting distribution enabled flag: %s", *cf.operationKey, err.Error())
		glog.Error(msg)
		curTask = curTaskFinished(curTask, StatusFailed, msg)
		err = svc.stg.UpdateDistributionStatus(*cf.distributionID, StatusFailed)
		return curTask, errors.New(msg)
	} else if *distOut.Distribution.Status != "Deployed" {
		curTask.Retries++
		glog.Infof("ActionEnableDistribution[%s]: retries: %3d status: %s", *cf.operationKey, curTask.Retries, *distOut.Distribution.Status)
		return curTask, nil
	}

	err = svc.stg.UpdateDistributionStatus(*cf.distributionID, StatusDeployed)
	curTask = curTaskFinished(curTask, StatusDeployed, "cloudfront distribution created and deployed")
	curTask.Action = NextAction[curTask.Action]

	return curTask, nil
}

func (svc *AwsConfig) ActionDeleteNew(cf *cloudFrontInstance) error {
	glog.Infof("===== ActionDeleteNew [%s] =====", *cf.operationKey)

	err := svc.stg.UpdateDistributionStatus(*cf.distributionID, StatusDisabling)
	if err != nil {
		msg := fmt.Sprintf("ActionDeleteNew: error updating distribution status: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	task := &storage.Task{
		DistributionID: *cf.distributionID,
		Action:         NextAction[ActionDeleteNew],
		Status:         StatusNew,
		Retries:        0,
		// OperationKey:   sql.NullString{String: *cf.operationKey, Valid: true},
		OperationKey: storage.SetNullString(*cf.operationKey),
		Result:       storage.SetNullString(OperationInProgress),
		Metadata:     storage.SetNullString(""),
	}

	task, err = svc.stg.AddTask(task)

	if err != nil {
		msg := fmt.Sprintf("ActionDeleteNew: error adding task: %s", err.Error())
		glog.Error(msg)
		return errors.New(msg)
	}

	return nil
}

func (svc *AwsConfig) ActionDisableDistribution(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Info("===== ActionDisableDistribution [%s] =====", *cf.operationKey)

	_, err := svc.getCloudfrontDistribution(cf)

	if err != nil {
		msg := fmt.Sprintf("ActionDisableDistribution [%s]: getting distribution from aws: %s", cf.operationKey, err.Error())
		curTask = curTaskFailed(curTask, "cloudfront distribution error")
		glog.Error(msg)
		return curTask, errors.New(msg)
	}

	err = svc.disableCloudfrontDistribution(cf)
	if err != nil {
		msg := fmt.Sprintf("ActionDisableDistribution [%s]: getting disabling distribution: %s", cf.operationKey, err.Error())
		curTask = curTaskFailed(curTask, "error disabling distribution")
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	curTask.Action = NextAction[curTask.Action]

	return curTask, nil
}

func (svc *AwsConfig) ActionDeleteOrigin(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Info("===== ActionDeleteOrigin [%s] =====", *cf.operationKey)

	err := svc.deleteS3Bucket(cf)
	if err != nil {
		msg := fmt.Sprintf("ActionDeleteOrigin [%s]: deleting s3 bucket: %s", cf.operationKey, err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	curTask.Action = NextAction[curTask.Action]
	return curTask, nil
}

func (svc *AwsConfig) ActionDeleteIAMUser(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Info("===== ActionDeleteIAMUser [%s] =====", *cf.operationKey)

	err := svc.deleteIAMUser(cf)
	if err != nil {
		msg := fmt.Sprintf("ActionDeleteIAMUser [%s]: deleting s3 bucket: %s", cf.operationKey, err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return curTask, nil
}

func (svc *AwsConfig) ActionDeleteDistribution(curTask *storage.Task, cf *cloudFrontInstance) (*storage.Task, error) {
	glog.Info("===== ActionDeleteDistribution [%s] =====", *cf.operationKey)

	err := svc.deleteIAMUser(cf)
	if err != nil {
		msg := fmt.Sprintf("ActionDeleteIAMUser [%s]: deleting s3 bucket: %s", cf.operationKey, err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return curTask, nil
}

// RunTasks is a go routine to run the actions in correct order.
// It will wait for AWS service to be available before going to next service
// Task status is in the tasks database table, so is safe to restart.
func (svc *AwsConfig) RunTasks() {
	ticker := time.NewTicker(time.Second * time.Duration(svc.waitSecs))

	glog.Info("===== RunTasks =====")
	for {
		var err error
		var cf *cloudFrontInstance
		var curTask *storage.Task

		<-ticker.C

		curTask, err = svc.stg.PopNextTask()

		if err != nil {
			if err == sql.ErrNoRows {
				glog.Error("RunTasks: no tasks")
				continue
			} else {
				msg := fmt.Sprintf("RunTask: error popping next task: %s", err.Error())
				glog.Error(msg)
				continue
			}
		}

		cf, err = svc.getCloudfrontInstance(curTask.DistributionID)
		cf.operationKey = &curTask.OperationKey.String

		switch curTask.Action {
		case ActionCreateOrigin:
			glog.Infof(">>>>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			now := time.Now()
			curTask.StartedAt = storage.SetNullTime(&now)
			curTask.Status = StatusPending
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				continue
			} else if curTask, err = svc.ActionCreateOrigin(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionCreateIAMUser:
			glog.Infof(">>>>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionCreateIAMUser(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionCreateAccessKey:
			glog.Infof(">>>>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionCreateAccessKey(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}

			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionCreateOriginAccessIdentity:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionCreateOriginAccessIdentity(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionCreateDistribution:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionCreateDistribution(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionAddBucketPolicy:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionAddBucketPolicy(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionIsDistributionDeployed:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionIsDistributionDeployed(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			} else {
				glog.Infof("RunTasks: ActionIsDistributionDeployed retries: %d", curTask.Retries)
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		// Deprovision actions

		case ActionDisableDistribution:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			now := time.Now()
			curTask.StartedAt = storage.SetNullTime(&now)
			curTask.Status = StatusPending
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				continue
			} else if curTask, err = svc.ActionDisableDistribution(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				continue
			}

		case ActionDeleteOrigin:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionDeleteOrigin(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionDeleteIAMUser:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			if curTask, err = svc.ActionDeleteIAMUser(curTask, cf); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
				curTask = curTaskFailed(curTask, err.Error())
			}
			if curTask, err = svc.stg.UpdateTaskAction(curTask); err != nil {
				msg := fmt.Sprintf("RunTask: error: %s", err.Error())
				glog.Error(msg)
			}

		case ActionDeleteDistribution:
			glog.Infof("action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			curTask = setTask(curTask, NextAction[curTask.Action], curTask.Status, nil, nil, false)
			curTask, err = svc.stg.UpdateTaskAction(curTask)

		case ActionDeleteOriginAccessIdentity:
			glog.Infof(">>>>> action: %s -> %s", curTask.Action, NextAction[curTask.Action])
			curTask = setTask(curTask, ActionDone, StatusDeleted, nil, nil, true)
			curTask, err = svc.stg.UpdateTaskAction(curTask)
		}
	}
}
