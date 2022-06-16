// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2022 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	sdk "github.com/edgexfoundry/device-sdk-go/v2/pkg/service"
	"net"
	"strings"
	"sync"
)

type MACAddressMapper struct {
	// credsMu is for locking access to the credsMap
	credsMu sync.RWMutex
	// credsMap is a map between mac address to secretPath
	credsMap map[string]string
}

func NewMACAddressMapper() *MACAddressMapper {
	return &MACAddressMapper{
		credsMap: make(map[string]string),
	}
}

// UpdateMappings takes the raw map of secret path to csv list of mac addresses and
// inverts it into a quick lookup map of mac address to secret path.
func (m *MACAddressMapper) UpdateMappings(raw map[string]string) {
	m.credsMu.Lock()
	defer m.credsMu.Unlock()

	lc := sdk.RunningService().LoggingClient

	credsMap := make(map[string]string)
	for secretPath, macs := range raw {
		if _, err := tryGetCredentials(secretPath); err != nil {
			lc.Warnf("One or more MAC address mappings exist for the secret path '%s' which does not exist in the Secret Store!", secretPath)
		}

		for _, mac := range strings.Split(macs, ",") {
			sanitized, err := SanitizeMACAddress(mac)
			if err != nil {
				lc.Warnf("Skipping entry: %s", err.Error())
				continue
			}
			// note: if the mac address already has a mapping, we do not overwrite it
			if existing, found := credsMap[sanitized]; found {
				lc.Warnf("Unable to set credential group to %s. MAC address '%s' already belongs to credential group %s.", secretPath, mac, existing)
			} else {
				credsMap[sanitized] = secretPath
			}
		}
	}

	m.credsMap = credsMap
}

// ListMACAddresses will return a slice of mac addresses that have been assigned credentials
func (m *MACAddressMapper) ListMACAddresses() []string {
	m.credsMu.RLock()
	defer m.credsMu.RUnlock()

	macs := make([]string, len(m.credsMap))

	i := 0
	for mac := range m.credsMap {
		macs[i] = mac
		i++
	}

	return macs
}

func (m *MACAddressMapper) GetSecretPathForMACAddress(mac string) (string, error) {
	m.credsMu.RLock()
	defer m.credsMu.RUnlock()

	// sanitize the mac address before looking up to ensure they all match the same format
	sanitized, err := SanitizeMACAddress(mac)
	if err != nil {
		return "", err
	}

	secretPath, ok := m.credsMap[sanitized]
	if !ok {
		return "", fmt.Errorf("no mapping exists for mac address '%s'", mac)
	}

	return secretPath, nil
}

func (m *MACAddressMapper) TryGetCredentialsForMACAddress(mac string) (Credentials, error) {
	secretPath, err := m.GetSecretPathForMACAddress(mac)
	if err != nil {
		return Credentials{}, err
	}
	return tryGetCredentials(secretPath)
}

// SanitizeMACAddress takes in a MAC address in one of the IEEE 802 MAC-48, EUI-48, EUI-64 formats
// and will return it in the standard go format, using colons and lower case letters:
// Example:	aa:bb:cc:dd:ee:ff
func SanitizeMACAddress(mac string) (string, error) {
	hwAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", fmt.Errorf("'%s' is not a valid MAC Address: %s", mac, err.Error())
	}
	return hwAddr.String(), nil
}
