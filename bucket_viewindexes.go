package gocb

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DesignDocumentNamespace represents which namespace a design document resides in.
type DesignDocumentNamespace bool

const (
	// ProductionDesignDocumentNamespace means that a design document resides in the production namespace.
	ProductionDesignDocumentNamespace = true

	// DevelopmentDesignDocumentNamespace means that a design document resides in the development namespace.
	DevelopmentDesignDocumentNamespace = false
)

// ViewIndexManager provides methods for performing View management.
// Volatile: This API is subject to change at any time.
type ViewIndexManager struct {
	bucket *Bucket

	tracer requestTracer
}

func (vm *ViewIndexManager) doMgmtRequest(req mgmtRequest) (*mgmtResponse, error) {
	resp, err := vm.bucket.executeMgmtRequest(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// View represents a Couchbase view within a design document.
type View struct {
	Map    string `json:"map,omitempty"`
	Reduce string `json:"reduce,omitempty"`
}

func (v View) hasReduce() bool {
	return v.Reduce != ""
}

// DesignDocument represents a Couchbase design document containing multiple views.
type DesignDocument struct {
	Name  string          `json:"-"`
	Views map[string]View `json:"views,omitempty"`
}

// GetDesignDocumentOptions is the set of options available to the ViewIndexManager GetDesignDocument operation.
type GetDesignDocumentOptions struct {
	Timeout       time.Duration
	RetryStrategy RetryStrategy
}

func (vm *ViewIndexManager) ddocName(name string, isProd DesignDocumentNamespace) string {
	if isProd {
		if strings.HasPrefix(name, "dev_") {
			name = strings.TrimLeft(name, "dev_")
		}
	} else {
		if !strings.HasPrefix(name, "dev_") {
			name = "dev_" + name
		}
	}

	return name
}

// GetDesignDocument retrieves a single design document for the given bucket.
func (vm *ViewIndexManager) GetDesignDocument(name string, namespace DesignDocumentNamespace, opts *GetDesignDocumentOptions) (*DesignDocument, error) {
	if opts == nil {
		opts = &GetDesignDocumentOptions{}
	}

	span := vm.tracer.StartSpan("GetDesignDocument", nil).SetTag("couchbase.service", "view")
	defer span.Finish()

	return vm.getDesignDocument(span.Context(), name, namespace, time.Now(), opts)
}

func (vm *ViewIndexManager) getDesignDocument(tracectx requestSpanContext, name string, namespace DesignDocumentNamespace,
	startTime time.Time, opts *GetDesignDocumentOptions) (*DesignDocument, error) {

	name = vm.ddocName(name, namespace)

	req := mgmtRequest{
		Service:       CapiService,
		Path:          fmt.Sprintf("/_design/%s", name),
		Method:        "GET",
		IsIdempotent:  true,
		RetryStrategy: opts.RetryStrategy,
		Timeout:       opts.Timeout,
	}
	resp, err := vm.doMgmtRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return nil, makeGenericMgmtError(ErrDesignDocumentNotFound, &req, resp)
		}

		return nil, makeMgmtBadStatusError("failed to get design document", &req, resp)
	}

	ddocObj := DesignDocument{}
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&ddocObj)
	if err != nil {
		return nil, err
	}

	ddocObj.Name = strings.TrimPrefix(name, "dev_")
	return &ddocObj, nil
}

// GetAllDesignDocumentsOptions is the set of options available to the ViewIndexManager GetAllDesignDocuments operation.
type GetAllDesignDocumentsOptions struct {
	Timeout       time.Duration
	RetryStrategy RetryStrategy
}

// GetAllDesignDocuments will retrieve all design documents for the given bucket.
func (vm *ViewIndexManager) GetAllDesignDocuments(namespace DesignDocumentNamespace, opts *GetAllDesignDocumentsOptions) ([]*DesignDocument, error) {
	if opts == nil {
		opts = &GetAllDesignDocumentsOptions{}
	}

	span := vm.tracer.StartSpan("GetAllDesignDocuments", nil).SetTag("couchbase.service", "view")
	defer span.Finish()

	req := mgmtRequest{
		Service:       MgmtService,
		Path:          fmt.Sprintf("/pools/default/buckets/%s/ddocs", vm.bucket.Name()),
		Method:        "GET",
		IsIdempotent:  true,
		Timeout:       opts.Timeout,
		RetryStrategy: opts.RetryStrategy,
	}
	resp, err := vm.doMgmtRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, makeMgmtBadStatusError("failed to get all design documents", &req, resp)
	}

	var ddocsObj struct {
		Rows []struct {
			Doc struct {
				Meta struct {
					ID string
				}
				JSON DesignDocument
			}
		}
	}
	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(&ddocsObj)
	if err != nil {
		return nil, err
	}

	var ddocs []*DesignDocument
	for index, ddocData := range ddocsObj.Rows {
		ddoc := &ddocsObj.Rows[index].Doc.JSON
		isProd := !strings.HasPrefix(ddoc.Name, "dev_")
		if isProd == bool(namespace) {
			ddoc.Name = strings.TrimPrefix(ddocData.Doc.Meta.ID[8:], "dev_")
			ddocs = append(ddocs, ddoc)
		}
	}

	return ddocs, nil
}

// UpsertDesignDocumentOptions is the set of options available to the ViewIndexManager UpsertDesignDocument operation.
type UpsertDesignDocumentOptions struct {
	Timeout       time.Duration
	RetryStrategy RetryStrategy
}

// UpsertDesignDocument will insert a design document to the given bucket, or update
// an existing design document with the same name.
func (vm *ViewIndexManager) UpsertDesignDocument(ddoc DesignDocument, namespace DesignDocumentNamespace, opts *UpsertDesignDocumentOptions) error {
	if opts == nil {
		opts = &UpsertDesignDocumentOptions{}
	}

	span := vm.tracer.StartSpan("UpsertDesignDocument", nil).SetTag("couchbase.service", "view")
	defer span.Finish()

	return vm.upsertDesignDocument(span.Context(), ddoc, namespace, time.Now(), opts)
}

func (vm *ViewIndexManager) upsertDesignDocument(
	tracectx requestSpanContext,
	ddoc DesignDocument,
	namespace DesignDocumentNamespace,
	startTime time.Time,
	opts *UpsertDesignDocumentOptions,
) error {
	espan := vm.tracer.StartSpan("encode", tracectx)
	data, err := json.Marshal(&ddoc)
	espan.Finish()
	if err != nil {
		return err
	}

	ddoc.Name = vm.ddocName(ddoc.Name, namespace)

	req := mgmtRequest{
		Service:       CapiService,
		Path:          fmt.Sprintf("/_design/%s", ddoc.Name),
		Method:        "PUT",
		Body:          data,
		Timeout:       opts.Timeout,
		RetryStrategy: opts.RetryStrategy,
	}
	resp, err := vm.doMgmtRequest(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return makeMgmtBadStatusError("failed to upsert design document", &req, resp)
	}

	return nil
}

// DropDesignDocumentOptions is the set of options available to the ViewIndexManager Upsert operation.
type DropDesignDocumentOptions struct {
	Timeout       time.Duration
	RetryStrategy RetryStrategy
}

// DropDesignDocument will remove a design document from the given bucket.
func (vm *ViewIndexManager) DropDesignDocument(name string, namespace DesignDocumentNamespace, opts *DropDesignDocumentOptions) error {
	if opts == nil {
		opts = &DropDesignDocumentOptions{}
	}

	span := vm.tracer.StartSpan("DropDesignDocument", nil).SetTag("couchbase.service", "view")
	defer span.Finish()

	return vm.dropDesignDocument(span.Context(), name, namespace, time.Now(), opts)
}

func (vm *ViewIndexManager) dropDesignDocument(tracectx requestSpanContext, name string, namespace DesignDocumentNamespace,
	startTime time.Time, opts *DropDesignDocumentOptions) error {

	name = vm.ddocName(name, namespace)

	req := mgmtRequest{
		Service:       CapiService,
		Path:          fmt.Sprintf("/_design/%s", name),
		Method:        "DELETE",
		Timeout:       opts.Timeout,
		RetryStrategy: opts.RetryStrategy,
	}
	resp, err := vm.doMgmtRequest(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return makeGenericMgmtError(ErrDesignDocumentNotFound, &req, resp)
		}

		return makeMgmtBadStatusError("failed to drop design document", &req, resp)
	}

	return nil
}

// PublishDesignDocumentOptions is the set of options available to the ViewIndexManager PublishDesignDocument operation.
type PublishDesignDocumentOptions struct {
	Timeout       time.Duration
	RetryStrategy RetryStrategy
}

// PublishDesignDocument publishes a design document to the given bucket.
func (vm *ViewIndexManager) PublishDesignDocument(name string, opts *PublishDesignDocumentOptions) error {
	startTime := time.Now()
	if opts == nil {
		opts = &PublishDesignDocumentOptions{}
	}

	span := vm.tracer.StartSpan("PublishDesignDocument", nil).
		SetTag("couchbase.service", "view")
	defer span.Finish()

	devdoc, err := vm.getDesignDocument(span.Context(), name, false, startTime, &GetDesignDocumentOptions{
		RetryStrategy: opts.RetryStrategy,
	})
	if err != nil {
		return err
	}

	err = vm.upsertDesignDocument(span.Context(), *devdoc, true, startTime, &UpsertDesignDocumentOptions{
		RetryStrategy: opts.RetryStrategy,
	})
	if err != nil {
		return err
	}

	return nil
}
