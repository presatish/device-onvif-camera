// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2022 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	sdk "github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"github.com/edgexfoundry/go-mod-bootstrap/v2/bootstrap/startup"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/errors"
)

type Credentials struct {
	Username string
	Password string
	AuthMode string
}

const (
	UsernameKey = "username"
	PasswordKey = "password"
	AuthModeKey = "mode"
)

// tryGetCredentials will attempt one time to get the credentials located at secretPath from
// secret provider and return them, otherwise return an error.
func tryGetCredentials(secretPath string) (Credentials, errors.EdgeX) {
	secretData, err := sdk.RunningService().SecretProvider.GetSecret(secretPath, UsernameKey, PasswordKey, AuthModeKey)
	if err != nil {
		return Credentials{}, errors.NewCommonEdgeXWrapper(err)
	}
	return Credentials{
		Username: secretData[UsernameKey],
		Password: secretData[PasswordKey],
		AuthMode: secretData[AuthModeKey],
	}, nil
}

// getCredentials will repeatedly try and get the credentials located at secretPath from
// secret provider every CredentialsRetryTime seconds for a maximum of CredentialsRetryWait seconds.
// Note that this function will block until either the credentials are found, or CredentialsRetryWait
// seconds have elapsed.
func (d *Driver) getCredentials(secretPath string) (credentials Credentials, err errors.EdgeX) {
	d.configMu.RLock()
	timer := startup.NewTimer(d.config.AppCustom.CredentialsRetryTime, d.config.AppCustom.CredentialsRetryWait)
	d.configMu.RUnlock()

	for timer.HasNotElapsed() {
		if credentials, err = tryGetCredentials(secretPath); err == nil {
			return credentials, nil
		}

		d.lc.Warnf(
			"Unable to retrieve camera credentials from SecretProvider at path '%s': %s. Retrying for %s",
			secretPath,
			err.Error(),
			timer.RemainingAsString())
		timer.SleepForInterval()
	}

	return credentials, err
}
