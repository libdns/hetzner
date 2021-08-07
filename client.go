package vercel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

type getAllRecordsResponse struct {
	Records []VercelRecord `json:"records"`
}

type createRecordResponse struct {
	Uid string `json:"uid"`
}

type VercelRecord struct {
	Id    string `json:"id,omitempty"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
}

func doRequest(token string, request *http.Request) ([]byte, error) {
	// Bearer Token for Vercel Authorization: Bearer <TOKEN>
	request.Header.Add("Authorization", "Bearer "+token)
	request.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		data, _ := ioutil.ReadAll(response.Body)
		return data, fmt.Errorf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode)
	}

	defer response.Body.Close()
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getAllRecords(ctx context.Context, token string, zone string) ([]libdns.Record, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.vercel.com/v4/domains/%s/records", zone), nil)
	if err != nil {
		return nil, err
	}

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
		records = append(records, libdns.Record{
			ID:    r.Id,
			Type:  r.Type,
			Name:  r.Name,
			Value: r.Value,
			TTL:   time.Duration(r.TTL) * time.Second,
		})
	}

	return records, nil
}

func createRecord(ctx context.Context, token string, zone string, r libdns.Record) (libdns.Record, error) {
	reqData := VercelRecord{
		Type:  r.Type,
		Name:  normalizeRecordName(r.Name, zone),
		Value: r.Value,
		TTL:   int(math.Max((r.TTL.Seconds()), 60)),
	}

	reqBuffer, err := json.Marshal(reqData)
	if err != nil {
		return libdns.Record{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://api.vercel.com/v2/domains/%s/records", zone), bytes.NewBuffer(reqBuffer))
	if err != nil {
		return libdns.Record{}, err
	}
	data, err := doRequest(token, req)
	if err != nil {
		return libdns.Record{}, err
	}

	result := createRecordResponse{}
	if err := json.Unmarshal(data, &result); err != nil {
		return libdns.Record{}, err
	}

	return libdns.Record{
		ID:    result.Uid,
		Type:  r.Type,
		Name:  normalizeRecordName(r.Name, zone),
		Value: r.Value,
	}, nil
}

func deleteRecord(ctx context.Context, zone string, token string, record libdns.Record) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://api.vercel.com/v2/domains/%s/records/%s", zone, record.ID), nil)
	if err != nil {
		return err
	}

	_, err = doRequest(token, req)
	if err != nil {
		return err
	}

	return nil
}

func updateRecord(ctx context.Context, token string, zone string, r libdns.Record) (libdns.Record, error) {
	err := deleteRecord(ctx, zone, token, r)

	if err != nil {
		return libdns.Record{}, err
	}

	newRecord, err := createRecord(ctx, token, zone, r)
	if err != nil {
		return libdns.Record{}, err
	}

	return newRecord, nil
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
