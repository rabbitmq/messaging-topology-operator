/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal_test

import (
	"net/url"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal Suite")
}

func mockRabbitMQServer(tls bool) *ghttp.Server {
	if tls {
		return ghttp.NewTLSServer()
	} else {
		return ghttp.NewServer()
	}
}

func mockRabbitMQURLPort(fakeRabbitMQServer *ghttp.Server) (*url.URL, int, error) {
	fakeRabbitMQURL, err := url.Parse(fakeRabbitMQServer.URL())
	if err != nil {
		return nil, 0, err
	}
	fakeRabbitMQPort, err := strconv.Atoi(fakeRabbitMQURL.Port())
	if err != nil {
		return nil, 0, err
	}
	return fakeRabbitMQURL, fakeRabbitMQPort, nil
}
