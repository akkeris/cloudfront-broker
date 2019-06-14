#!/bin/sh
GLOG_logtostderr=1 ./cloudfront-broker -insecure -logtostderr=1 -stderrthreshold 0 $*
