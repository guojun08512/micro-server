package sealclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"keyayun.com/seal-micro-runner/pkg/logger"

	fsdk "git.keyayun.com/bohaoc/seal-file-sdk"
	pb "keyayun.com/seal-micro-runner/pkg/proto"
	"keyayun.com/seal-micro-runner/pkg/redis"
)

const timeout = 60

var (
	lgr   = logger.WithNamespace("sealclient")
	limit = 1000
)

// SealClient Model
type SealClient struct {
	fsdk.SealClient
	Token *pb.TokenModel
}

// RefreshTokenResp model
type RefreshTokenResp struct {
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	AccessToken string `json:"access_token"`
}

type ServerStatus struct {
	CouchDB string `json:"couchdb"`
	Message string `json:"message"`
}

func (s *SealClient) wrapSetRequestHeaders(method, urlPath string) (*fsdk.Options, error) {
	opts, err := s.SetRequestHeaders(method, urlPath)
	if opts != nil {
		opts.Authorizer = fsdk.Authorizer(&fsdk.BearerAuthorizer{
			Token: s.Token.AccessToken,
		})
	}
	return opts, err
}

func (s *SealClient) FindDataDoc(docType string, param url.Values) ([]*fsdk.SealDoc, error) {
	options, err := s.wrapSetRequestHeaders(http.MethodGet, fmt.Sprintf("/v1/data/%s/find", docType))
	if err != nil {
		lgr.Errorf("FindDataDoc failed when wrapSetRequestHeaders: %v", err)
		return nil, err
	}
	if param != nil {
		options.Queries = param
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		return nil, err
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		lgr.Errorf("FindDataDoc failed when read response body: %v", err)
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest || resp.StatusCode < http.StatusOK {
		lgr.Errorf("FindDataDoc failed as response code is `%d`: %s", resp.StatusCode, string(bytes))
		return nil, fmt.Errorf("FindDataDoc failed as response code is `%d`: %s", resp.StatusCode, string(bytes))
	}
	if len(bytes) == 0 {
		return []*fsdk.SealDoc{}, nil
	}
	var payloads fsdk.DataPayloads
	err = json.Unmarshal(bytes, &payloads)
	if err != nil {
		lgr.Errorf("FindDataDoc failed when parse json: %v", err)
		return nil, err
	}
	return payloads.Data, nil
}

func (s *SealClient) GetAllDataDocs(docType string) ([]*fsdk.SealDoc, error) {
	urlPath := fmt.Sprintf("/v1/data/%s/all", docType)
	body, err := s.querySealData(urlPath, nil)
	if err != nil {
		lgr.Errorf("GetAllDataDocs failed when querySealData:%v", err)
		return nil, err
	}
	var payloads fsdk.DataPayloads
	err = json.Unmarshal(body, &payloads)
	if err != nil {
		lgr.Errorf("GetAllDataDocs failed when Unmarshal: %v", err)
		return nil, err
	}

	if len(payloads.Data) == 0 {
		lgr.Error("GetAllDataDocs Payload is empty")
		return nil, errors.New("Payload is empty")
	}
	return payloads.Data, nil
}

// GetDataDoc Method
func (s *SealClient) GetDataDoc(docType, docID string) (*fsdk.SealDoc, error) {
	path := fmt.Sprintf("/v1/data/%s/%s", docType, docID)
	body, err := s.querySealData(path, nil)
	if err != nil {
		lgr.Errorf("GetDataDoc failed when querySealData:%v", err)
		return nil, err
	}
	var payloads fsdk.DataPayloads
	err = json.Unmarshal(body, &payloads)
	if err != nil {
		lgr.Errorf("querySealData failed when Unmarshal:%v", err)
		return nil, err
	}

	if len(payloads.Data) == 0 {
		lgr.Error("GetDataDoc Payload is empty")
		return nil, errors.New("Payload is empty")
	}

	return payloads.Data[0], nil
}

// GetAndUpdateDataDoc Method
func (s *SealClient) GetAndUpdateDataDoc(docType, docID string, update map[string]interface{}) (*fsdk.SealDoc, error) {
	doc, err := s.GetDataDoc(docType, docID)
	if err != nil {
		lgr.Errorf("GetAndUpdateDataDoc failed when GetDataDoc:%v", err)
		return nil, err
	}
	if doc != nil && doc.ID == "" && doc.Attr == nil {
		return nil, errors.New("doc not found")
	}
	if doc.Attr == nil {
		doc.Attr = make(map[string]interface{})
	}
	for k, v := range update {
		doc.Attr[k] = v
	}
	doc, err = s.UpdateDataDoc(doc)
	if err != nil {
		lgr.Errorf("GetAndUpdateDataDoc failed when update doc: %v", err)
		return nil, err
	}
	return doc, err
}

// UpdateDataDoc Method
func (s *SealClient) UpdateDataDoc(doc *fsdk.SealDoc) (*fsdk.SealDoc, error) {
	path := fmt.Sprintf("/v1/data/%s/%s", doc.Type, doc.ID)
	body, err := s.updateSealData(path, []*fsdk.SealDoc{doc})
	if err != nil {
		lgr.Errorf("UpdateDataDoc failed when updateSealData:%v", err)
		return nil, err
	}
	var payloads fsdk.DataPayloads
	err = json.Unmarshal(body, &payloads)
	if err != nil {
		lgr.Errorf("UpdateDataDoc failed when Unmarshal:%v", err)
		return nil, err
	}

	if len(payloads.Data) == 0 {
		lgr.Error("UpdateDataDoc Payload is empty")
		return nil, errors.New("payload is empty")
	}

	return payloads.Data[0], nil
}

func (s *SealClient) querySealData(urlpath string, params url.Values) ([]byte, error) {
	options, err := s.wrapSetRequestHeaders(http.MethodGet, urlpath)
	if err != nil {
		lgr.Errorf("QuerySealData failed when do wrapSetRequestHeaders: %v", err)
		return nil, err
	}
	if params != nil {
		options.Queries = params
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		lgr.Errorf("QuerySealData failed when do request: %v", err)
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		lgr.Errorf("QuerySealData failed when read response body: %v", err)
		return nil, err
	}
	return body, nil
}

func (s *SealClient) updateSealData(urlpath string, docs []*fsdk.SealDoc) ([]byte, error) {
	options, err := s.wrapSetRequestHeaders(http.MethodPut, urlpath)
	if err != nil {
		lgr.Errorf("updateSealData failed when wrapSetRequestHeaders: %v", err)
		return nil, err
	}

	if docs != nil {
		reqData, err := WriteJSON(fsdk.DataPayloads{Data: docs})
		if err != nil {
			lgr.Errorf("UpdateSealData failed when WriteJSON:%v", err)
			return nil, err
		}
		options.Body = reqData
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return body, nil
}

// CreateDataDoc Method
func (s *SealClient) CreateDataDoc(docType string, doc *fsdk.SealDoc) (*fsdk.SealDoc, error) {
	docs, err := s.CreateDataDocs(docType, []*fsdk.SealDoc{doc})
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, errors.New("payload is empty")
	}
	return docs[0], nil
}

// CreateDataDocs Method
func (s *SealClient) CreateDataDocs(docType string, docs []*fsdk.SealDoc) ([]*fsdk.SealDoc, error) {
	options, err := s.wrapSetRequestHeaders(http.MethodPost, fmt.Sprintf("/v1/data/%s/create", docType))
	if err != nil {
		lgr.Errorf("CreateDataDocs failed when wrapSetRequestHeaders: %v", err)
		return nil, err
	}
	if docs != nil {
		reqData, err := WriteJSON(&fsdk.DataPayloads{Data: docs})
		if err != nil {
			lgr.Errorf("CreateDataDocs failed when WriteJSON:%v", err)
			return nil, err
		}
		options.Body = reqData
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var payloads fsdk.DataPayloads
	err = json.Unmarshal(body, &payloads)
	if err != nil {
		lgr.Errorf("CreateDataDocs failed when Unmarshal:%v", err)
		return nil, err
	}

	if len(payloads.Data) == 0 {
		lgr.Error("Payload is empty")
		return nil, errors.New("Payload is empty")
	}

	return payloads.Data, nil
}

func (s *SealClient) GetReference(targetDocType, targetDocID, referenceType string) ([]*fsdk.SealDoc, error) {
	options, err := s.wrapSetRequestHeaders(http.MethodGet, fmt.Sprintf("/v1/references/%s/%s", targetDocType, targetDocID))
	if err != nil {
		lgr.Errorf("GetReference failed when wrapSetRequestHeaders: %v", err)
		return nil, err
	}
	param := url.Values{
		"doctype": []string{referenceType},
	}
	options.Queries = param
	resp, err := fsdk.Req(options)
	if err != nil {
		lgr.Errorf("GetReference failed when do request: %v", err)
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest || resp.StatusCode < http.StatusOK {
		lgr.Errorf("GetReference failed as response code is `%d`: %s", resp.StatusCode, resp.Status)
		return nil, fmt.Errorf("GetReference failed as response code is `%d`: %s", resp.StatusCode, resp.Status)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		lgr.Errorf("GetReference failed when read response body: %v", err)
		return nil, err
	}
	if len(bytes) == 0 {
		return []*fsdk.SealDoc{}, nil
	}
	var payloads fsdk.DataPayloads
	err = json.Unmarshal(bytes, &payloads)
	if err != nil {
		lgr.Errorf("GetReference failed when parse json: %v", err)
		return nil, err
	}
	return payloads.Data, nil
}

// GetOrCreateReference Method
func (s *SealClient) GetOrCreateReference(targetDocType, targetDocID, referenceType string, linkDoc *fsdk.SealDoc) (*fsdk.SealDoc, error) {
	docs, err := s.GetReference(targetDocType, targetDocID, referenceType)
	if err != nil {
		lgr.Errorf("GetOrCreateReference failed when get reference: %v", err)
		return nil, err
	}
	if len(docs) > 0 {
		return docs[0], nil
	}
	err = s.CreateReference(targetDocType, targetDocID, linkDoc)
	if err != nil {
		lgr.Errorf("GetOrCreateReference failed when CreateReference: %v", err)
		return nil, err
	}
	return linkDoc, nil
}

// CreateReference Method
func (s *SealClient) CreateReference(targetDocType, targetDocID string, linkDoc *fsdk.SealDoc) error {
	options, err := s.wrapSetRequestHeaders(http.MethodPost, fmt.Sprintf("/v1/references/%s/%s", targetDocType, targetDocID))
	if err != nil {
		lgr.Errorf("CreateReference failed when wrapSetRequestHeaders: %v", err)
		return err
	}
	reqData, err := WriteJSON(&fsdk.DataPayloads{Data: []*fsdk.SealDoc{linkDoc}})
	if err != nil {
		lgr.Errorf("CreateReference failed when WriteJSON: %v", err)
		return err
	}
	options.Body = reqData
	resp, err := fsdk.Req(options)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("create reference failed as response code is `%d`: %s", resp.StatusCode, resp.Status)
	}
	return nil
}

func (s *SealClient) GetDirInfoByDirID(dirID string) (*fsdk.SealDoc, error) {
	path := fmt.Sprintf("/files/%s", dirID)
	q := url.Values{
		"page[limit]": {"1"},
	}
	body, err := s.querySealData(path, q)
	if err != nil {
		return nil, fmt.Errorf("requestCozyFilesByDirID failed when QuerySealData: %v", err)
	}
	var resp *fsdk.DataPayload
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, fmt.Errorf("requestCozyFilesByDirID failed when unmarshal json: %v", err)
	}
	return resp.Data, nil
}

//DeleteDocsByDocIDs 删除docs
func (s *SealClient) DeleteDocsByDocIDs(doctype string, docs []*fsdk.SealDoc) error {
	urlpath := fmt.Sprintf("/v1/data/%s/%s", doctype, docs[0].ID)
	options, err := s.wrapSetRequestHeaders(http.MethodDelete, urlpath)
	if err != nil {
		lgr.Errorf("DeleteDocsByDocIDs failed when wrapSetRequestHeaders: %v", err)
		return err
	}
	if docs != nil {
		reqData, err := WriteJSON(fsdk.DataPayloads{Data: docs})
		if err != nil {
			lgr.Errorf("DeleteDocsByDocIDs failed when WriteJSON:%v", err)
			return err
		}
		options.Body = reqData
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			lgr.Errorf("DeleteDocsByDocIDs failed when read response body: %v", err)
			return err
		}
		return fmt.Errorf("DeleteDocsByDocIDs file failed: %s", string(b))
	}
	return nil
}

//UpdateDocsByDocIDs 更新docs
func (s *SealClient) UpdateDocsByDocIDs(doctype string, docs []*fsdk.SealDoc) error {
	urlpath := fmt.Sprintf("/v1/data/%s/%s", doctype, docs[0].ID)
	options, err := s.wrapSetRequestHeaders(http.MethodPut, urlpath)
	if err != nil {
		lgr.Errorf("UpdateDocsByDocIDs failed when wrapSetRequestHeaders: %v", err)
		return err
	}
	if docs != nil {
		reqData, err := WriteJSON(fsdk.DataPayloads{Data: docs})
		if err != nil {
			lgr.Errorf("UpdateDocsByDocIDs failed when WriteJSON:%v", err)
			return err
		}
		options.Body = reqData
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			lgr.Errorf("UpdateDocsByDocIDs failed when read response body: %v", err)
			return err
		}
		return fmt.Errorf("UpdateDocsByDocIDs file failed: %s", string(b))
	}
	return nil
}

// index查询
func (s *SealClient) FindDocsByIndex(doctype, indexName string, args ...map[string]interface{}) ([]*fsdk.SealDoc, error) {
	path := fmt.Sprintf("/v1/data/%s/find", doctype)
	selector := make(map[string]interface{})
	for _, arg := range args {
		for k, v := range arg {
			selector[k] = v
		}
	}
	bsSelector, err := json.Marshal(selector)
	if err != nil {
		return nil, err
	}
	offset := 0
	var docs []*fsdk.SealDoc
	for {
		q := url.Values{
			"index":       {indexName},
			"selector":    {string(bsSelector)},
			"page[limit]": {strconv.Itoa(limit)},
			"page[skip]":  {strconv.Itoa(offset)},
		}
		body, err := s.querySealData(path, q)
		var payloads fsdk.DataPayloads
		err = json.Unmarshal(body, &payloads)
		if err != nil {
			lgr.Errorf("FindDocsByIndex failed when Unmarshal:%v", err)
			return nil, err
		}
		if len(payloads.Data) > 0 {
			docs = append(docs, payloads.Data...)
		}
		offset = offset + limit
		if payloads.Links.Next == "" {
			break
		}
	}
	return docs, nil
}

func (s *SealClient) DownloadFileByDicomweb(studyInstanceUID, seriesInstanceUID, sopInstanceUID, workPath string) error {
	uri := fmt.Sprintf("/dicom-web/studies/%s/series/%s/instances/%s", studyInstanceUID, seriesInstanceUID, sopInstanceUID)
	options, err := s.wrapSetRequestHeaders(http.MethodGet, uri)
	if err != nil {
		lgr.Errorf("DownloadFileByDicomweb failed when do wrapSetRequestHeaders: %v", err)
		return err
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		lgr.Errorf("DownloadFileByDicomweb failed when do request: %v", err)
		return err
	}
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		defer resp.Body.Close()
		mr := multipart.NewReader(resp.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			slurp, err := ioutil.ReadAll(p)
			if err != nil {
				return err
			}
			return ioutil.WriteFile(workPath, slurp, os.ModePerm)
		}
	}
	return nil
}

// 查询series下所有的instance
func (s *SealClient) GetInstancesCountByDicomweb(studyInstanceUID, seriesInstanceUID string) (int, error) {
	uri := fmt.Sprintf("/dicom-web/studies/%s/series/%s/instances/count", studyInstanceUID, seriesInstanceUID)
	options, err := s.wrapSetRequestHeaders(http.MethodGet, uri)
	if err != nil {
		lgr.Errorf("DownloadFileByDicomweb failed when do wrapSetRequestHeaders: %v", err)
		return 0, err
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		lgr.Errorf("DownloadFileByDicomweb failed when do request: %v", err)
		return 0, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var res map[string]interface{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return 0, err
	}
	count := res["count"].(float64)
	return int(count), nil
}

func (s *SealClient) GetAllInstancesByDicomweb(studyInstanceUID, seriesInstanceUID string) ([]map[string]interface{}, error) {
	count, err := s.GetInstancesCountByDicomweb(studyInstanceUID, seriesInstanceUID)
	if err != nil {
		lgr.Errorf("DownloadFileByDicomweb failed when do GetInstancesCountByDicomweb: %v", err)
		return nil, err
	}
	var res []map[string]interface{}
	offset := 0
	limit := 100
	for {
		uri := fmt.Sprintf("/dicom-web/studies/%s/series/%s/instances", studyInstanceUID, seriesInstanceUID)
		options, err := s.wrapSetRequestHeaders(http.MethodGet, uri)
		if err != nil {
			lgr.Errorf("DownloadFileByDicomweb failed when do wrapSetRequestHeaders: %v", err)
			return nil, err
		}
		query := url.Values{
			"offset": []string{strconv.Itoa(offset)},
			"limit":  []string{strconv.Itoa(limit)},
		}
		options.Queries = query
		resp, err := fsdk.Req(options)
		if err != nil {
			lgr.Errorf("DownloadFileByDicomweb failed when do request: %v", err)
			return nil, err
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var docs []map[string]interface{}
		err = json.Unmarshal(body, &docs)
		if err != nil {
			return nil, err
		}
		res = append(res, docs...)
		if len(res) >= count {
			break
		}
		offset += limit
	}
	return res, nil
}

// workitems 查询
func (s *SealClient) GetWorkItemByID(workItemID string) (map[string]interface{}, error) {
	uriPath := fmt.Sprintf("/workitems/%s", workItemID)
	options, err := s.wrapSetRequestHeaders(http.MethodGet, uriPath)
	if err != nil {
		lgr.Errorf("GetWorkItemByID failed when wrapSetRequestHeaders: %v", err)
		return nil, err
	}
	resp, err := fsdk.Req(options)
	if err != nil {
		lgr.Errorf("GetWorkItemByID failed when do request: %v", err)
		return nil, err
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		lgr.Errorf("GetWorkItemByID failed when read response body: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	var payload map[string]interface{}
	err = json.Unmarshal(bytes, &payload)
	if err != nil {
		lgr.Errorf("GetWorkItemByID failed when Unmarshal: %v", err)
		return nil, err
	}
	return payload, nil
}

// 更新
func (s *SealClient) UpdateWorkItem(workItemID string, body map[string]interface{}) error {
	uriPath := fmt.Sprintf("/workitems/%s", workItemID)
	options, err := s.wrapSetRequestHeaders(http.MethodPost, uriPath)
	if err != nil {
		lgr.Errorf("UpdateWorkItem failed when wrapSetRequestHeaders: %v", err)
		return err
	}
	bReader, err := WriteJSON(body)
	if err != nil {
		return err
	}
	options.Body = bReader
	_, err = fsdk.Req(options)
	if err != nil {
		lgr.Errorf("UpdateWorkItem failed when do request: %v", err)
		return err
	}
	return nil
}

func (s *SealClient) UpdateWorkItemState(workItemID string, body map[string]interface{}) error {
	uriPath := fmt.Sprintf("/workitems/%s/state", workItemID)
	options, err := s.wrapSetRequestHeaders(http.MethodPut, uriPath)
	if err != nil {
		lgr.Errorf("UpdateWorkItemState failed when wrapSetRequestHeaders: %v", err)
		return err
	}
	bReader, err := WriteJSON(body)
	if err != nil {
		return err
	}
	options.Body = bReader
	_, err = fsdk.Req(options)
	if err != nil {
		lgr.Errorf("UpdateWorkItemState failed when do request: %v", err)
		return err
	}
	return nil
}

func (s *SealClient) CreateWorkItem(body map[string]interface{}) error {
	affectedUID := fmt.Sprintf("1.3976.1.20.%d", time.Now().UnixNano())
	url := url.URL{
		Scheme:   s.Scheme,
		Path:     fmt.Sprintf("/workitems"),
		Host:     s.Domain,
		RawQuery: affectedUID,
	}
	bReader, err := WriteJSON(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bReader)
	if err != nil {
		lgr.Errorf("CreateWorkItem failed when create request: %v", err)
		return err
	}
	var httpClient = http.Client{
		Timeout: timeout * time.Second,
	}
	req.Header = map[string][]string{
		"Authorization": {s.Authorizer.AuthHeader()},
		"Content-Type":  {"application/json"},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		lgr.Errorf("CreateWorkItem failed when do request: %v", err)
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		errStr := fmt.Sprintf("CreateWorkItem failed as response code is `%d`", resp.StatusCode)
		lgr.Error(errStr)
		return errors.New(errStr)
	}
	return nil
}

// CheckServerStatus func
func CheckServerStatus(domain string) error {
	flag := false
	for _, scheme := range []string{"http", "https"} {
		uri := fmt.Sprintf("%s://%s/status", scheme, domain)
		req, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			lgr.Errorf("CheckServerStatus(scheme: %s, domain: %s) failed when create request: %v", scheme, scheme, err)
			continue
		}
		var httpClient = http.Client{
			Timeout: timeout * time.Second,
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lgr.Errorf("CheckServerStatus(scheme: %s, domain: %s) failed when do request: %v", scheme, domain, err)
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
			errStr := fmt.Sprintf("CheckServerStatus(scheme: %s, domain: %s) failed as response code is `%d`", scheme, domain, resp.StatusCode)
			lgr.Error(errStr)
			return errors.New(errStr)
		}
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			lgr.Errorf("CheckServerStatus failed when read response body: %v", err)
			return err
		}
		lgr.Debugf("CheckServerStatus: %s", string(bytes))
		flag = true
		break
	}
	if flag {
		return nil
	}
	return errors.New("no accessible server")
}

// RefreshToken 刷新Token
func RefreshToken() error {
	oldTokens, err := redis.GetRunnerInstances()
	if err != nil {
		lgr.Errorf("RefreshToken when GetRunnerInstances: %v", err)
		return err
	}
	for serviceName, old := range oldTokens {
		uri := fmt.Sprintf("%s://%s%s", old.Scheme, old.Domain, old.RefreshUri) // RefreshURI 不需要更新
		req, err := http.NewRequest("POST", uri, nil)
		if err != nil {
			lgr.Errorf("RefreshToken(domain: %s) failed when create request: %v", old.Domain, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Host = old.Domain
		var httpClient = http.Client{
			Timeout: timeout * time.Second,
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lgr.Errorf("RefreshToken(domain: %s) failed when do request: %v", old.Domain, err)
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
			body, _ := ioutil.ReadAll(resp.Body)
			lgr.Errorf("RefreshToken(domain: %s) failed as response code is `%d`: %s", old.Domain, resp.StatusCode, string(body))
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			lgr.Errorf("RefreshToken(domain: %s) failed when read response body: %v", old.Domain, err)
			continue
		}

		rt := new(RefreshTokenResp)
		err = json.Unmarshal(body, rt)
		if err != nil {
			lgr.Errorf("RefreshToken(domain: %s) failed when unmarshal json: %v", old.Domain, err)
			continue
		}
		lgr.Infof("RefreshToken(domain: %s) successfully!", old.Domain)
		old.AccessToken = rt.AccessToken

		err = redis.RegisterInstance(serviceName, old)
		if err != nil {
			lgr.Errorf("RefreshToken(domain: %s) RegisterInstance error: %v", old.Domain, err)
			continue
		}
		_ = resp.Body.Close()
	}

	return nil
}
