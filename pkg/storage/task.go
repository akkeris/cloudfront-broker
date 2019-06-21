package storage

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	// pq "github.com/lib/pq"
)

// Status's
const (
	StatusNew      string = "new"
	StatusPending  string = "pending"
	StatusFinished string = "finished"
	StatusFailed   string = "failed"
)

// AddTask inserts task into tasks table
func (p *PostgresStorage) AddTask(task *Task) (*Task, error) {
	var err error

	glog.Info("===== AddTask =====")

	err = p.db.QueryRow(insertTaskScript, &task.DistributionID, &task.Status, &task.Action, &task.OperationKey, &task.Retries, &task.StartedAt).Scan(&task.TaskID)

	if err != nil {
		msg := fmt.Sprintf("AddTask: error adding task: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return task, nil
}

// GetTaskByDistribution retrieves task by distribution id
func (p *PostgresStorage) GetTaskByDistribution(distributionID string) (*Task, error) {
	glog.Infof("===== GetTaskByDistribution [%s] =====", distributionID)
	task := Task{}

	err := p.db.QueryRow(selectTaskScript, distributionID).Scan(&task.TaskID, &task.DistributionID, &task.OperationKey, &task.Status, &task.Action, &task.Retries, &task.Metadata, &task.Result)

	if err != nil {
		msg := fmt.Sprintf("GetTaskByDistribution: error finding task: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	return &task, nil
}

// PopNextTask retrieves next task from tasks table
func (p *PostgresStorage) PopNextTask() (*Task, error) {
	var err error
	var task Task

	glog.Info("===== PopNextTask =====")
	err = p.db.QueryRow(popNextTaskScript, StatusPending).Scan(&task.TaskID, &task.DistributionID, &task.OperationKey, &task.Status, &task.Action, &task.Retries, &task.Metadata, &task.Result, &task.StartedAt, &task.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// UpdateTaskAction updates a task in the db
func (p *PostgresStorage) UpdateTaskAction(task *Task) (*Task, error) {
	var err error

	glog.Info("===== UpdateTaskAction =====")

	err = p.db.QueryRow(updateTaskActionScript, task.TaskID, task.Action, task.Status, task.Retries, task.Result, task.Metadata, task.FinishedAt, task.StartedAt).Scan(
		&task.TaskID, &task.DistributionID, &task.Action, &task.Status, &task.Retries, &task.Result, &task.Metadata, &task.CreatedAt, &task.UpdatedAt, &task.StartedAt, &task.FinishedAt)

	if err != nil {
		msg := fmt.Sprintf("UpdateTaskAction: error updating task: %s", err.Error())
		glog.Error(msg)
		return nil, errors.New(msg)
	}

	if task == nil {
		glog.Error("UpdateTaskAction: task is nil")
		return nil, errors.New("UpdateTaskAction: task is nil")
	}
	return task, nil
}
