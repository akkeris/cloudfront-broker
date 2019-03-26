package storage

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	// pq "github.com/lib/pq"
)

const (
	StatusNew      string = "new"
	StatusPending  string = "pending"
	StatusFinished string = "finished"
	StatusFailed   string = "failed"
)

func (p *PostgresStorage) AddTask(task *Task) (*Task, error) {
	var err error
	status := StatusNew

	glog.Info("===== AddTask =====")

	err = p.db.QueryRow(`
    insert into tasks
    (task_id, distribution_id, status, action, operation_key, retries, started_at)
    values 
    (uuid_generate_v4(), $1, $2, $3, $4, $5, $6) returning task_id
  `, &task.DistributionID, &status, &task.Action, &task.OperationKey, &task.Retries, &task.StartedAt).Scan(&task.TaskID)

	if err != nil {
		msg := fmt.Sprintf("AddTask: error adding task: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return task, nil
}

func (p *PostgresStorage) GetTaskByDistribution(distributionID string, operationKey string) (*Task, error) {
	glog.Infof("===== GetTaskByDistribution [%s] =====", operationKey)
	task := Task{}

	var selectTaskScript = `
  select task_id, distribution_id, operation_key, status, action, retries, metadata, result
  where distribution_id = $1
  and operation_key = $2
  and delete_at is null
  and finished_at is null
`
	err := p.db.QueryRow(selectTaskScript, distributionID).Scan(&task.TaskID, &task.DistributionID, &task.OperationKey, &task.Status, &task.Action, &task.Retries, &task.Metadata, &task.Result)

	if err != nil {
		msg := fmt.Sprintf("GetTaskByDistribution [%s]: error selecting task: %s", err.Error())
		glog.Info(msg)
		return nil, errors.New(msg)
	}

	return &task, nil
}

func (p *PostgresStorage) PopNextTask() (*Task, error) {
	var err error
	var task Task

	glog.Info("===== PopNextTask =====")
	err = p.db.QueryRow(`
        update tasks set
            status = $1,
            updated_at = now() 
        where 
            task_id in ( select task_id
                     from tasks 
                     where status in ('new', 'pending') 
                     and deleted_at is null 
                     and finished_at is null 
                     order by updated_at asc limit 1 )
        returning task_id, distribution_id, operation_key, status, action, retries, metadata, result, started_at, updated_at
    `, StatusPending).Scan(&task.TaskID, &task.DistributionID, &task.OperationKey, &task.Status, &task.Action, &task.Retries, &task.Metadata, &task.Result, &task.StartedAt, &task.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

func (p *PostgresStorage) UpdateTaskAction(task *Task) (*Task, error) {
	var err error

	glog.Info("===== UpdateTaskAction =====")

	err = p.db.QueryRow(`
    UPDATE tasks set
      action = $2,
      status = $3, 
      retries = $4,
      result = $5,
      metadata = $6,
      finished_at = $7,
      updated_at = now()
    WHERE task_id = $1
    AND finished_at is null
    AND deleted_at is null
    RETURNING task_id, distribution_id, action, status, retries, result, metadata, created_at, updated_at, started_at, finished_at
`, task.TaskID, task.Action, task.Status, task.Retries, task.Result, task.Metadata, task.FinishedAt).Scan(
		&task.TaskID, &task.DistributionID, &task.Action, &task.Status, &task.Retries, &task.Result, &task.Metadata, &task.CreatedAt, &task.UpdatedAt, &task.StartedAt, &task.FinishedAt)

	if err != nil {
		msg := fmt.Sprintf("UpdateTaskAction: error updating task: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}
	return task, nil
}
