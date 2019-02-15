// Author: ned.hanks
// Date Created: ned.hanks
// Project:
package utils

import (
  "time"

  "github.com/nu7hatch/gouuid"
)

var OnePtr *int64
var TwoPtr *int64
var TtlPtr *int64

func TruePtr() *bool {
	b := true
	return &b
}

func FalsePtr() *bool {
	b := false
	return &b
}

func StrPtr(s string) *string {
	r := s
	return &r
}

func Int64Ptr(n int64) *int64 {
	r := n
	return &r
}

func NowPtr() *time.Time {
	n := time.Now()
	return &n
}

func NewUuid() string {
	u, _ := uuid.NewV4()
	s := u.String()
	return s
}

func NewUuidPtr() *string {
	return StrPtr(NewUuid())
}

func Init() {
	OnePtr = Int64Ptr(1)
	TwoPtr = Int64Ptr(2)
	TtlPtr = Int64Ptr(2592000)
}
