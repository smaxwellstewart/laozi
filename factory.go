package laozi

import (
	"bytes"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// LoggerFactory is an interface that defines how to make a new logger.
// This Logger will be responsible for logging all events to it that match the same
// partition key.
type LoggerFactory interface {
	NewLogger(key string) Logger
}

type S3LoggerFactory struct {
	Prefix string
	Bucket string
	Region string
}

func (lf S3LoggerFactory) NewLogger(key string) Logger {
	l := &s3logger{
		bucket: lf.Bucket,
		key:    fmt.Sprintf("%s%s", lf.Prefix, key),
		S3:     s3.New(session.New(), &aws.Config{Region: aws.String(lf.Region)}),
		buffer: bytes.NewBuffer([]byte{}),
		active: time.Now(),
	}

	l.fetchPreviousData()

	return l

}