package yrequest

import (
	"bytes"
	"fmt"
	"github.com/moul/http2curl"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

type Job struct {
	Url                                        string
	Method                                     string
	Payload                                    []byte
	Headers                                    map[string]string
	RetryIfError                               time.Duration
	SleepInCaseOfError                         time.Duration
	CurlStr                                    string
	Info                                       string
	Delay                                      time.Duration
	LockBetweenReqs                            time.Duration
}

type Result struct {
	Job                     Job
	Body                    []byte
	Error                   error
	StatusCode              int
	Header                  http.Header
	Response                *http.Response
	Proxy                   string
	ContentType             string
}


func Response(job *Job, requestTimeout time.Duration) (*Result, error) {
	t1 := time.Now()
	client := &http.Client{
		Timeout: requestTimeout,
	}
	req, err := http.NewRequest(job.Method, job.Url, bytes.NewBuffer(job.Payload))
	if err != nil {
		errN := errors.Wrap(err, "couldn't create request")
		return nil, errN
	}
	req.Close = true
	for k, v := range job.Headers {
		req.Header.Add(k, v)
	}
	curlCmd, toCurlErr := http2curl.GetCurlCommand(req)
	if toCurlErr != nil {
		logrus.Errorf("err: %+v", toCurlErr)
	}
	job.CurlStr = curlCmd.String()
	l := logrus.WithField("job-info", job.Info)

	logrus.Tracef("requesting %+v", job)
	res, err := client.Do(req)
	defer func() {
		logrus.Tracef("request is finished for %s: %+v", time.Since(t1), job)
	}()
	if err != nil {
		err = errors.Wrapf(err, "couldn't make request for req %s and timeout: %s, time spent: %+v",
			job.CurlStr, requestTimeout, time.Since(t1))
		l.Tracef("error: %+v", err)
		return nil, err
	}
	l.Tracef("request is done: %+v", res.StatusCode)

	defer res.Body.Close()
	b, err2 := io.ReadAll(res.Body)
	if err2 != nil {
		return nil, errors.WithStack(err)
	}
	if res.StatusCode > 399 || res.StatusCode < 200 {
		bodyText := string(b)
		const maxBodyTextChars = 2500
		var bodyTextForErr string
		if utf8.RuneCountInString(bodyText) > maxBodyTextChars {
			bodyTextForErr = string([]rune(bodyText)[:maxBodyTextChars])
		}
		errB := errors.WithStack(fmt.Errorf("not good status code %+v requesting %+v, bodyTextForErr: %+v", res.StatusCode, job, bodyTextForErr))
		return &Result{Job: *job, Body: []byte(bodyText), StatusCode: res.StatusCode, Header: res.Header, Response: res}, errB
	}
	return &Result{Job: *job, StatusCode: res.StatusCode, Header: res.Header, Response: res, Body: b}, nil
}

