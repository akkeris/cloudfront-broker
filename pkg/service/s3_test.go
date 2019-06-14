// Author: ned.hanks
// Date Created: December 7, 2018
// Project:
package service

import (
	"testing"

	"cloudfront-broker/pkg/storage"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

func TestAwsConfig_deleteS3Bucket(t *testing.T) {
	type fields struct {
		namePrefix string
		conf       *aws.Config
		sess       *session.Session
		waitSecs   int64
		maxRetries int64
		stg        *storage.PostgresStorage
	}
	type args struct {
		cf *cloudFrontInstance
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AwsConfig{
				namePrefix: tt.fields.namePrefix,
				conf:       tt.fields.conf,
				sess:       tt.fields.sess,
				waitSecs:   tt.fields.waitSecs,
				maxRetries: tt.fields.maxRetries,
				stg:        tt.fields.stg,
			}
			if err := s.deleteS3Bucket(tt.args.cf); (err != nil) != tt.wantErr {
				t.Errorf("AwsConfig.deleteS3Bucket() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
