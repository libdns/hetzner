package hetzner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

type getAllRecordsResponse struct {
	Records []record `json:"records"`
}

type getAllZonesResponse struct {
	Zones []zone `json:"zones"`
}

type createRecordResponse struct {
	Record record `json:"record"`
}

type updateRecordResponse struct {
	Record record `json:"record"`
}

type zone struct {
	ID  string `json:"id"`
	TTL int    `json:"ttl"`
}

type record struct {
	ID     string `json:"id,omitempty"`
	ZoneID string `json:"zone_id,omitempty"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	TTL    *int   `json:"ttl"`
}

func doRequest(token string, request *http.Request) ([]byte, error) {
	request.Header.Add("Auth-API-Token", token)

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode)
	}

	defer response.Body.Close()
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getZoneData(ctx context.Context, token string, name string) (zone, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://dns.hetzner.com/api/v1/zones?name=%s", url.QueryEscape(name)), nil)
	data, err := doRequest(token, req)
	if err != nil {
		return zone{}, err
	}

	result := getAllZonesResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return zone{}, err
	}

	if len(result.Zones) > 1 {
		return zone{}, errors.New("zone is ambiguous")
	}

	return result.Zones[0], nil
}

func getAllRecords(ctx context.Context, token string, zone string) ([]libdns.Record, error) {
	zoneData, err := getZoneData(ctx, token, zone)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://dns.hetzner.com/api/v1/records?zone_id=%s", zoneData.ID), nil)
	data, err := doRequest(token, req)
	if err != nil {
		return nil, err
	}

	result := getAllRecordsResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	records := []libdns.Record{}
	for _, r := range result.Records {
		rec := libdns.Record{
			ID:    r.ID,
			Type:  r.Type,
			Name:  r.Name,
			Value: r.Value,
		}
		if r.TTL != nil {
			rec.TTL = time.Duration(*r.TTL) * time.Second
		} else {
			rec.TTL = time.Duration(zoneData.TTL) * time.Second
		}
		records = append(records, rec)
	}

	return records, nil
}

func createRecord(ctx context.Context, token string, zone string, r libdns.Record) (libdns.Record, error) {
	zoneData, err := getZoneData(ctx, token, zone)
	if err != nil {
		return libdns.Record{}, err
	}

	reqData := record{
		ZoneID: zoneData.ID,
		Type:   r.Type,
		Name:   normalizeRecordName(r.Name, zone),
		Value:  r.Value,
		TTL:    ptr(int(r.TTL.Seconds())),
	}

	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return libdns.Record{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://dns.hetzner.com/api/v1/records", bytes.NewBuffer(reqBuffer))
	data, err := doRequest(token, req)
	if err != nil {
		return libdns.Record{}, err
	}

	result := createRecordResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return libdns.Record{}, err
	}

	rec := libdns.Record{
		ID:    result.Record.ID,
		Type:  result.Record.Type,
		Name:  result.Record.Name,
		Value: result.Record.Value,
	}
	if result.Record.TTL != nil {
		rec.TTL = time.Duration(*result.Record.TTL) * time.Second
	} else {
		rec.TTL = time.Duration(zoneData.TTL) * time.Second
	}
	return rec, nil
}

func deleteRecord(ctx context.Context, token string, record libdns.Record) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://dns.hetzner.com/api/v1/records/%s", record.ID), nil)
	_, err = doRequest(token, req)
	if err != nil {
		return err
	}

	return nil
}

func updateRecord(ctx context.Context, token string, zone string, r libdns.Record) (libdns.Record, error) {
	zoneData, err := getZoneData(ctx, token, zone)
	if err != nil {
		return libdns.Record{}, err
	}

	reqData := record{
		ZoneID: zoneData.ID,
		Type:   r.Type,
		Name:   normalizeRecordName(r.Name, zone),
		Value:  r.Value,
		TTL:    ptr(int(r.TTL.Seconds())),
	}

	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return libdns.Record{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("https://dns.hetzner.com/api/v1/records/%s", r.ID), bytes.NewBuffer(reqBuffer))
	data, err := doRequest(token, req)
	if err != nil {
		return libdns.Record{}, err
	}

	result := updateRecordResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return libdns.Record{}, err
	}

	rec := libdns.Record{
		ID:    result.Record.ID,
		Type:  result.Record.Type,
		Name:  result.Record.Name,
		Value: result.Record.Value,
	}
	if result.Record.TTL != nil {
		rec.TTL = time.Duration(*result.Record.TTL) * time.Second
	} else {
		rec.TTL = time.Duration(zoneData.TTL) * time.Second
	}
	return rec, nil
}

func createOrUpdateRecord(ctx context.Context, token string, zone string, r libdns.Record) (libdns.Record, error) {
	if len(r.ID) == 0 {
		return createRecord(ctx, token, zone, r)
	}

	return updateRecord(ctx, token, zone, r)
}

func normalizeRecordName(recordName string, zone string) string {
	// Workaround for https://github.com/caddy-dns/hetzner/issues/3
	// Can be removed after https://github.com/libdns/libdns/issues/12
	normalized := unFQDN(recordName)
	normalized = strings.TrimSuffix(normalized, unFQDN(zone))
	return unFQDN(normalized)
}

func ptr(val int) *int {
	return &val
}
