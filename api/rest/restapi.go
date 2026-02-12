// Package rest implements an IPFS Cluster API component. It provides
// a REST-ish API to interact with Cluster.
//
// The implented API is based on the common.API component (refer to module
// description there). The only thing this module does is to provide route
// handling for the otherwise common API component.
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/ipfs-cluster/ipfs-cluster/adder/adderutils"
	types "github.com/ipfs-cluster/ipfs-cluster/api"
	"github.com/ipfs-cluster/ipfs-cluster/api/common"

	logging "github.com/ipfs/go-log/v2"
	rpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"

	mux "github.com/gorilla/mux"
)

var (
	logger    = logging.Logger("restapi")
	apiLogger = logging.Logger("restapilog")
)

type peerAddBody struct {
	PeerID string `json:"peer_id"`
}

// API implements the REST API Component.
// It embeds a common.API.
type API struct {
	*common.API

	rpcClient *rpc.Client
	config    *Config
}

// NewAPI creates a new REST API component.
func NewAPI(ctx context.Context, cfg *Config) (*API, error) {
	return NewAPIWithHost(ctx, cfg, nil)
}

// NewAPIWithHost creates a new REST API component using the given libp2p Host.
func NewAPIWithHost(ctx context.Context, cfg *Config, h host.Host) (*API, error) {
	api := API{
		config: cfg,
	}
	capi, err := common.NewAPIWithHost(ctx, &cfg.Config, h, api.routes)
	api.API = capi
	return &api, err
}

// Routes returns endpoints supported by this API.
func (api *API) routes(c *rpc.Client) []common.Route {
	api.rpcClient = c
	return []common.Route{
		{
			Name:        "ID",
			Method:      "GET",
			Pattern:     "/id",
			HandlerFunc: api.idHandler,
		},

		{
			Name:        "Version",
			Method:      "GET",
			Pattern:     "/version",
			HandlerFunc: api.versionHandler,
		},

		{
			Name:        "Peers",
			Method:      "GET",
			Pattern:     "/peers",
			HandlerFunc: api.peerListHandler,
		},
		{
			Name:        "PeerAdd",
			Method:      "POST",
			Pattern:     "/peers",
			HandlerFunc: api.peerAddHandler,
		},
		{
			Name:        "PeerRemove",
			Method:      "DELETE",
			Pattern:     "/peers/{peer}",
			HandlerFunc: api.peerRemoveHandler,
		},
		{
			Name:        "Add",
			Method:      "POST",
			Pattern:     "/add",
			HandlerFunc: api.addHandler,
		},
		{
			Name:        "Allocations",
			Method:      "GET",
			Pattern:     "/allocations",
			HandlerFunc: api.allocationsHandler,
		},
		{
			Name:        "Allocation",
			Method:      "GET",
			Pattern:     "/allocations/{hash}",
			HandlerFunc: api.allocationHandler,
		},
		{
			Name:        "StatusAll",
			Method:      "GET",
			Pattern:     "/pins",
			HandlerFunc: api.statusAllHandler,
		},
		{
			Name:        "Recover",
			Method:      "POST",
			Pattern:     "/pins/{hash}/recover",
			HandlerFunc: api.recoverHandler,
		},
		{
			Name:        "RecoverAll",
			Method:      "POST",
			Pattern:     "/pins/recover",
			HandlerFunc: api.recoverAllHandler,
		},
		{
			Name:        "Status",
			Method:      "GET",
			Pattern:     "/pins/{hash}",
			HandlerFunc: api.statusHandler,
		},
		{
			Name:        "Pin",
			Method:      "POST",
			Pattern:     "/pins/{hash}",
			HandlerFunc: api.pinHandler,
		},
		{
			Name:        "PinPath",
			Method:      "POST",
			Pattern:     "/pins/{keyType:ipfs|ipns|ipld}/{path:.*}",
			HandlerFunc: api.pinPathHandler,
		},
		{
			Name:        "Unpin",
			Method:      "DELETE",
			Pattern:     "/pins/{hash}",
			HandlerFunc: api.unpinHandler,
		},
		{
			Name:        "UnpinPath",
			Method:      "DELETE",
			Pattern:     "/pins/{keyType:ipfs|ipns|ipld}/{path:.*}",
			HandlerFunc: api.unpinPathHandler,
		},
		{
			Name:        "UpdatePinMetadata",
			Method:      "PATCH",
			Pattern:     "/pins/{cid}/metadata",
			HandlerFunc: api.updatePinMetadataHandler,
		},
		{
			Name:        "RepoGC",
			Method:      "POST",
			Pattern:     "/ipfs/gc",
			HandlerFunc: api.repoGCHandler,
		},
		{
			Name:        "ConnectionGraph",
			Method:      "GET",
			Pattern:     "/health/graph",
			HandlerFunc: api.graphHandler,
		},
		{
			Name:        "Alerts",
			Method:      "GET",
			Pattern:     "/health/alerts",
			HandlerFunc: api.alertsHandler,
		},
		{
			Name:        "Bandwidth by protocol stats",
			Method:      "GET",
			Pattern:     "/health/bandwidth",
			HandlerFunc: api.bandwidthByProtocolHandler,
		},
		{
			Name:        "Metrics",
			Method:      "GET",
			Pattern:     "/monitor/metrics/{name}",
			HandlerFunc: api.metricsHandler,
		},
		{
			Name:        "MetricNames",
			Method:      "GET",
			Pattern:     "/monitor/metrics",
			HandlerFunc: api.metricNamesHandler,
		},
		{
			Name:        "GetToken",
			Method:      "POST",
			Pattern:     "/token",
			HandlerFunc: api.GenerateTokenHandler,
		},
		{
			Name:        "Health",
			Method:      "GET",
			Pattern:     "/health",
			HandlerFunc: api.HealthHandler,
		},
	}
}

func (api *API) idHandler(w http.ResponseWriter, r *http.Request) {
	var id types.ID
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"ID",
		struct{}{},
		&id,
	)

	api.SendResponse(w, common.SetStatusAutomatically, err, &id)
}

func (api *API) versionHandler(w http.ResponseWriter, r *http.Request) {
	var v types.Version
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Version",
		struct{}{},
		&v,
	)

	api.SendResponse(w, common.SetStatusAutomatically, err, v)
}

func (api *API) graphHandler(w http.ResponseWriter, r *http.Request) {
	var graph types.ConnectGraph
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"ConnectGraph",
		struct{}{},
		&graph,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, graph)
}

func (api *API) metricsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var metrics []types.Metric
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"PeerMonitor",
		"LatestMetrics",
		name,
		&metrics,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, metrics)
}

func (api *API) metricNamesHandler(w http.ResponseWriter, r *http.Request) {
	var metricNames []string
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"PeerMonitor",
		"MetricNames",
		struct{}{},
		&metricNames,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, metricNames)
}

func (api *API) alertsHandler(w http.ResponseWriter, r *http.Request) {
	var alerts []types.Alert
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Alerts",
		struct{}{},
		&alerts,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, alerts)
}

func (api *API) bandwidthByProtocolHandler(w http.ResponseWriter, r *http.Request) {
	var bw types.BandwidthByProtocol
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"BandwidthByProtocol",
		struct{}{},
		&bw,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, bw)
}

func (api *API) addHandler(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, err, nil)
		return
	}

	params, err := types.AddParamsFromQuery(r.URL.Query())
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, err, nil)
		return
	}

	// Extract metadata from multipart form field if present
	// We need to read through the multipart to find the metadata field
	// and reconstruct it without that field for the adder
	metadataJSON, newReader, hasFiles, err := extractMetadataFromMultipart(reader)
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, fmt.Errorf("error extracting metadata: %w", err), nil)
		return
	}

	// Ensure we have at least one file
	if !hasFiles {
		api.SendResponse(w, http.StatusBadRequest, errors.New("no files found in multipart form"), nil)
		return
	}

	// Parse metadata if found
	if metadataJSON != "" {
		var rawMetadata any
		if err := json.Unmarshal([]byte(metadataJSON), &rawMetadata); err != nil {
			api.SendResponse(w, http.StatusBadRequest, fmt.Errorf("error parsing metadata JSON: %w", err), nil)
			return
		}
		// Convert to map[string]any, handling nested objects properly
		metadata := normalizeMetadata(rawMetadata)
		if metadata == nil {
			api.SendResponse(w, http.StatusBadRequest, errors.New("metadata must be a JSON object"), nil)
			return
		}
		// Merge metadata from form body with existing metadata (form body takes precedence)
		if params.Metadata == nil {
			params.Metadata = make(map[string]any)
		}
		for k, v := range metadata {
			params.Metadata[k] = v
		}
		api.config.Logger.Infof("addHandler: extracted and merged metadata from form body: %+v", params.Metadata)
	} else {
		api.config.Logger.Debugf("addHandler: no metadata found in form body, using query params metadata: %+v", params.Metadata)
	}

	api.config.Logger.Debugf("addHandler: final params before adding: Metadata=%+v, NoPin=%v", params.Metadata, params.NoPin)

	api.SetHeaders(w)

	// Use the reconstructed reader (without metadata field)
	// any errors sent as trailer
	rootCid, err := adderutils.AddMultipartHTTPHandler(
		r.Context(),
		api.rpcClient,
		params,
		newReader,
		w,
		nil,
	)
	if err != nil {
		api.config.Logger.Errorf("addHandler: error adding content: %v", err)
		// Error is already sent as trailer by AddMultipartHTTPHandler
		return
	}
	api.config.Logger.Infof("addHandler: successfully added content with root CID: %s, NoPin: %v", rootCid, params.NoPin)
}

// normalizeMetadata recursively converts map[interface{}]interface{} to map[string]any
// and []interface{} to []any to ensure JSON serialization works correctly.
func normalizeMetadata(v any) map[string]any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		// Already the correct type, but normalize nested values
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = normalizeValue(v)
		}
		pruneEmptyMaps(result)
		return result
	case map[interface{}]interface{}:
		// Convert map[interface{}]interface{} to map[string]any
		result := make(map[string]any, len(val))
		for k, v := range val {
			key, ok := k.(string)
			if !ok {
				key = fmt.Sprintf("%v", k)
			}
			result[key] = normalizeValue(v)
		}
		pruneEmptyMaps(result)
		return result
	default:
		return nil
	}
}

// pruneEmptyMaps removes keys whose value is an empty map (e.g. "observation": {}).
// Nested maps are pruned first so a parent can become empty and then be removed.
// Modifies m in place.
func pruneEmptyMaps(m map[string]any) {
	if m == nil {
		return
	}
	for k, v := range m {
		if child, ok := v.(map[string]any); ok {
			pruneEmptyMaps(child)
			if len(child) == 0 {
				delete(m, k)
			}
		}
	}
}

// normalizeValue recursively normalizes a value to ensure it can be JSON serialized.
func normalizeValue(v any) any {
	if v == nil {
		return nil
	}

	// Check if it's a byte slice and convert to string
	// Note: []byte and []uint8 are the same type in Go, so we only need one case
	switch val := v.(type) {
	case []byte:
		return string(val)
	}

	// Use reflection to handle slices generically since []interface{} and []any are the same
	if reflect.TypeOf(v).Kind() == reflect.Slice {
		rv := reflect.ValueOf(v)
		// Check if it's a byte slice via reflection
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			// It's a []byte or []uint8, convert to string
			return string(rv.Bytes())
		}
		// Check if it's []interface{} or []any containing only uint8 values (byte array)
		if rv.Len() > 0 {
			allUint8 := true
			bytes := make([]byte, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				elem := rv.Index(i).Interface()
				switch val := elem.(type) {
				case uint8:
					bytes = append(bytes, val)
				case int:
					// Check if it's a valid byte value (0-255)
					if val >= 0 && val <= 255 {
						bytes = append(bytes, byte(val))
					} else {
						allUint8 = false
						break
					}
				default:
					allUint8 = false
					break
				}
			}
			if allUint8 && len(bytes) > 0 {
				// It's a byte array, convert to string
				return string(bytes)
			}
		}
		// Not a byte array, process as regular slice
		result := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result[i] = normalizeValue(rv.Index(i).Interface())
		}
		return result
	}

	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = normalizeValue(v)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]any, len(val))
		for k, v := range val {
			key, ok := k.(string)
			if !ok {
				key = fmt.Sprintf("%v", k)
			}
			result[key] = normalizeValue(v)
		}
		return result
	default:
		// Primitive types (string, number, bool) are already JSON-serializable
		return v
	}
}

// extractMetadataFromMultipart reads through the multipart form to find and extract
// the metadata field, then reconstructs a new multipart reader without the metadata field.
// Returns the metadata JSON string (empty if not found), the new reader, whether files were found, and any error.
func extractMetadataFromMultipart(reader *multipart.Reader) (string, *multipart.Reader, bool, error) {
	var metadataJSON string
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	hasFiles := false
	fileCount := 0
	fieldCount := 0

	// Read through all parts
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writer.Close()
			return "", nil, false, fmt.Errorf("error reading multipart: %w", err)
		}

		formName := part.FormName()
		fileName := part.FileName()

		// Check if this is the metadata field
		if formName == "metadata" && fileName == "" {
			// Read the metadata value
			metadataBytes, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				writer.Close()
				return "", nil, false, fmt.Errorf("error reading metadata field: %w", err)
			}
			metadataJSON = string(metadataBytes)
			continue // Skip adding this part to the reconstructed form
		}

		// Copy this part to the new multipart form
		if fileName != "" {
			// It's a file
			hasFiles = true
			fileCount++
			partWriter, err := writer.CreateFormFile(formName, fileName)
			if err != nil {
				part.Close()
				writer.Close()
				return "", nil, false, fmt.Errorf("error creating form file: %w", err)
			}
			copied, err := io.Copy(partWriter, part)
			part.Close()
			if err != nil {
				writer.Close()
				return "", nil, false, fmt.Errorf("error copying file data: %w", err)
			}
			if copied == 0 {
				writer.Close()
				return "", nil, false, fmt.Errorf("file %s has zero bytes", fileName)
			}
		} else if formName != "" {
			// It's a form field (but not metadata, which we already handled)
			fieldCount++
			fieldValue, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				writer.Close()
				return "", nil, false, fmt.Errorf("error reading form field: %w", err)
			}
			if err := writer.WriteField(formName, string(fieldValue)); err != nil {
				writer.Close()
				return "", nil, false, fmt.Errorf("error writing form field: %w", err)
			}
		} else {
			// Unknown part type, close it and continue
			part.Close()
		}
	}

	err := writer.Close()
	if err != nil {
		return "", nil, false, fmt.Errorf("error closing multipart writer: %w", err)
	}

	// Verify buffer has content if we expected files
	if buf.Len() == 0 && hasFiles {
		return "", nil, false, fmt.Errorf("multipart buffer is empty but files were expected")
	}

	// Log reconstruction summary (using apiLogger from package)
	apiLogger.Debugf("extractMetadataFromMultipart: reconstructed multipart with %d files, %d fields, buffer size: %d bytes", fileCount, fieldCount, buf.Len())

	// Create a new multipart reader from the buffer
	// The buffer is automatically positioned at the start
	boundary := writer.Boundary()
	newReader := multipart.NewReader(&buf, boundary)

	return metadataJSON, newReader, hasFiles, nil
}

func (api *API) peerListHandler(w http.ResponseWriter, r *http.Request) {
	in := make(chan struct{})
	close(in)
	out := make(chan types.ID, common.StreamChannelSize)
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		errCh <- api.rpcClient.Stream(
			r.Context(),
			"",
			"Cluster",
			"Peers",
			in,
			out,
		)
	}()

	iter := func() (interface{}, bool, error) {
		p, ok := <-out
		return p, ok, nil
	}
	api.StreamResponse(w, iter, errCh)
}

func (api *API) peerAddHandler(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var addInfo peerAddBody
	err := dec.Decode(&addInfo)
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, errors.New("error decoding request body"), nil)
		return
	}

	pid, err := peer.Decode(addInfo.PeerID)
	if err != nil {
		api.SendResponse(w, http.StatusBadRequest, errors.New("error decoding peer_id"), nil)
		return
	}

	var id types.ID
	err = api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"PeerAdd",
		pid,
		&id,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, &id)
}

func (api *API) peerRemoveHandler(w http.ResponseWriter, r *http.Request) {
	if p := api.ParsePidOrFail(w, r); p != "" {
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"PeerRemove",
			p,
			&struct{}{},
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, nil)
	}
}

func (api *API) pinHandler(w http.ResponseWriter, r *http.Request) {
	if pin := api.ParseCidOrFail(w, r); pin.Defined() {
		api.config.Logger.Debugf("rest api pinHandler: %s", pin.Cid)
		// span.AddAttributes(trace.StringAttribute("cid", pin.Cid))
		var pinObj types.Pin
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"Pin",
			pin,
			&pinObj,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, pinObj)
		api.config.Logger.Debug("rest api pinHandler done")
	}
}

func (api *API) unpinHandler(w http.ResponseWriter, r *http.Request) {
	if pin := api.ParseCidOrFail(w, r); pin.Defined() {
		api.config.Logger.Debugf("rest api unpinHandler: %s", pin.Cid)
		// span.AddAttributes(trace.StringAttribute("cid", pin.Cid))
		var pinObj types.Pin
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"Unpin",
			pin,
			&pinObj,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, pinObj)
		api.config.Logger.Debug("rest api unpinHandler done")
	}
}

func (api *API) pinPathHandler(w http.ResponseWriter, r *http.Request) {
	var pin types.Pin
	if pinpath := api.ParsePinPathOrFail(w, r); pinpath.Defined() {
		api.config.Logger.Debugf("rest api pinPathHandler: %s", pinpath.Path)
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"PinPath",
			pinpath,
			&pin,
		)

		api.SendResponse(w, common.SetStatusAutomatically, err, pin)
		api.config.Logger.Debug("rest api pinPathHandler done")
	}
}

func (api *API) unpinPathHandler(w http.ResponseWriter, r *http.Request) {
	var pin types.Pin
	if pinpath := api.ParsePinPathOrFail(w, r); pinpath.Defined() {
		api.config.Logger.Debugf("rest api unpinPathHandler: %s", pinpath.Path)
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"UnpinPath",
			pinpath,
			&pin,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, pin)
		api.config.Logger.Debug("rest api unpinPathHandler done")
	}
}

func (api *API) updatePinMetadataHandler(w http.ResponseWriter, r *http.Request) {
	api.config.Logger.Info("updatePinMetadataHandler: starting metadata update request")

	// 1️⃣ Get CID from URL
	vars := mux.Vars(r)
	cidStr := vars["cid"]
	if cidStr == "" {
		api.config.Logger.Error("updatePinMetadataHandler: CID is empty")
		api.SendResponse(w, http.StatusBadRequest, errors.New("cid is required"), nil)
		return
	}

	c, err := types.DecodeCid(cidStr)
	if err != nil {
		api.config.Logger.Errorf("updatePinMetadataHandler: failed to decode CID: %v", err)
		api.SendResponse(w, http.StatusBadRequest, fmt.Errorf("invalid CID: %w", err), nil)
		return
	}
	api.config.Logger.Debugf("updatePinMetadataHandler: successfully decoded CID: %s", c)

	// 2️⃣ Decode JSON body containing metadata updates
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		api.config.Logger.Errorf("updatePinMetadataHandler: failed to decode JSON body: %v", err)
		api.SendResponse(w, http.StatusBadRequest, fmt.Errorf("invalid JSON body: %w", err), nil)
		return
	}
	defer r.Body.Close()

	if len(updates) == 0 {
		api.config.Logger.Warn("updatePinMetadataHandler: no metadata updates provided")
		api.SendResponse(w, http.StatusBadRequest, errors.New("no metadata updates provided"), nil)
		return
	}
	api.config.Logger.Debugf("updatePinMetadataHandler: received metadata updates: %+v", updates)

	// 3️⃣ Fetch existing pin via RPC
	var pinObj types.Pin
	err = api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"PinGet", // RPC method in Cluster
		c,        // pass types.Cid, not string
		&pinObj,
	)
	if err != nil {
		api.config.Logger.Errorf("updatePinMetadataHandler: failed to fetch pin: %v", err)
		api.SendResponse(w, http.StatusNotFound, fmt.Errorf("pin not found: %w", err), nil)
		return
	}
	api.config.Logger.Debugf("updatePinMetadataHandler: fetched pin metadata: %+v", pinObj.Metadata)

	// 4️⃣ Merge existing metadata with updates
	if pinObj.Metadata == nil {
		pinObj.Metadata = make(map[string]any)
	}

	for k, v := range updates {
		pinObj.Metadata[k] = v
	}
	api.config.Logger.Debugf("updatePinMetadataHandler: merged metadata: %+v", pinObj.Metadata)

	// 5️⃣ Update pin via RPC
	var updatedPin types.Pin
	err = api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"Pin",  // Reuse Pin RPC to update metadata
		pinObj, // pass full pin object with updated metadata
		&updatedPin,
	)
	if err != nil {
		api.config.Logger.Errorf("updatePinMetadataHandler: failed to update pin metadata: %v", err)
		api.SendResponse(w, http.StatusInternalServerError, fmt.Errorf("failed to update pin metadata: %w", err), nil)
		return
	}

	// 6️⃣ Return updated pin
	api.config.Logger.Infof("updatePinMetadataHandler: metadata update successful for CID: %s", c)
	api.SendResponse(w, http.StatusOK, nil, updatedPin)
}

func (api *API) allocationsHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	filterStr := queryValues.Get("filter")
	var filter types.PinType
	for _, f := range strings.Split(filterStr, ",") {
		filter |= types.PinTypeFromString(f)
	}

	if filter == types.BadType {
		api.SendResponse(w, http.StatusBadRequest, errors.New("invalid filter value"), nil)
		return
	}

	in := make(chan struct{})
	close(in)

	out := make(chan types.Pin, common.StreamChannelSize)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer close(errCh)

		errCh <- api.rpcClient.Stream(
			r.Context(),
			"",
			"Cluster",
			"Pins",
			in,
			out,
		)
	}()

	iter := func() (interface{}, bool, error) {
		var p types.Pin
		var ok bool
	iterloop:
		for {

			select {
			case <-ctx.Done():
				break iterloop
			case p, ok = <-out:
				if !ok {
					break iterloop
				}
				// this means we keep iterating if no filter
				// matched
				if filter == types.AllType || filter&p.Type > 0 {
					break iterloop
				}
			}
		}
		return p, ok, ctx.Err()
	}

	api.StreamResponse(w, iter, errCh)
}

func (api *API) allocationHandler(w http.ResponseWriter, r *http.Request) {
	if pin := api.ParseCidOrFail(w, r); pin.Defined() {
		var pinResp types.Pin
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"PinGet",
			pin.Cid,
			&pinResp,
		)
		api.SendResponse(w, common.SetStatusAutomatically, err, pinResp)
	}
}

func (api *API) statusAllHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Debug: log raw URL so we can verify query string reaches the handler
	logger.Infof("statusAllHandler: request URL RawQuery='%s'", r.URL.RawQuery)

	queryValues := r.URL.Query()
	if queryValues.Get("cids") != "" {
		api.statusCidsHandler(w, r)
		return
	}

	local := queryValues.Get("local")

	// Parse status filter
	filterStr := queryValues.Get("filter")
	statusFilter := types.TrackerStatusFromString(filterStr)
	logger.Infof("statusAllHandler: filterStr='%s', statusFilter=%s (value=%d), local=%s", filterStr, statusFilter.String(), statusFilter, local)

	// FIXME: This is a bit lazy, as "invalidxx,pinned" would result in a
	// valid "pinned" filter.
	if statusFilter == types.TrackerStatusUndefined && filterStr != "" {
		logger.Warnf("statusAllHandler: Invalid filter value: '%s'", filterStr)
		api.SendResponse(w, http.StatusBadRequest, errors.New("invalid filter value"), nil)
		return
	}

	// Parse metadata filter (format: "key1:value1,key2:value2" or "key:[min,max]")
	metadataStr := queryValues.Get("metadata")
	if metadataStr == "" {
		// Fallback: case-insensitive or raw query (some proxies/routers alter Query())
		for k, v := range queryValues {
			if strings.EqualFold(k, "metadata") && len(v) > 0 {
				metadataStr = v[0]
				break
			}
		}
	}
	if metadataStr == "" && strings.Contains(r.URL.RawQuery, "metadata=") {
		// Fallback: parse from raw query if Query() lost it
		raw, _ := url.ParseQuery(r.URL.RawQuery)
		metadataStr = raw.Get("metadata")
	}
	logger.Infof("statusAllHandler: metadataStr='%s'", metadataStr)

	// Use raw string so filter works after RPC (map may not deserialize over the wire)
	filter := types.NewStatusFilterWithMetadataStr(statusFilter, metadataStr)

	// Parse metadata filter once for API-side filtering (used in closure; re-parsing is safe if metadataStr is set)
	metadataFilter := types.MetadataFilterFromString(metadataStr)
	hasMetadataFilter := len(metadataFilter) > 0
	logger.Infof("statusAllHandler: hasMetadataFilter=%v, metadataFilter=%+v", hasMetadataFilter, metadataFilter)

	var iter common.StreamIterator
	in := make(chan types.StatusFilter, 1)
	in <- filter
	close(in)
	errCh := make(chan error, 1)

	logger.Debugf("statusAllHandler: Sending filter (status=%s, metadata=%v) to RPC", filter.Status.String(), filter.Metadata)

	if local == "true" {
		out := make(chan types.PinInfo, common.StreamChannelSize)
		count := 0
		iter = func() (interface{}, bool, error) {
			for {
				select {
				case <-ctx.Done():
					return nil, false, ctx.Err()
				case p, ok := <-out:
					if !ok {
						logger.Infof("statusAllHandler: Finished streaming %d items (local)", count)
						return nil, false, nil
					}
					// Normalize metadata to ensure JSON serialization works
					if p.Metadata != nil {
						p.Metadata = normalizeMetadata(p.Metadata)
					}
					gpi := p.ToGlobal()
					// Also normalize in GlobalPinInfo to be safe
					if gpi.Metadata != nil {
						gpi.Metadata = normalizeMetadata(gpi.Metadata)
					}
					// Apply metadata filter on API side using pre-parsed filter
					if hasMetadataFilter {
						sf := types.StatusFilter{Metadata: metadataFilter}
						if !sf.MatchMetadata(gpi.Metadata) {
							continue
						}
					}
					count++
					if count%10 == 0 || count <= 5 {
						logger.Debugf("statusAllHandler: Streamed %d items (local)", count)
					}
					return gpi, true, nil
				}
			}
		}

		go func() {
			defer close(errCh)

			errCh <- api.rpcClient.Stream(
				r.Context(),
				"",
				"Cluster",
				"StatusAllLocal",
				in,
				out,
			)
		}()

	} else {
		out := make(chan types.GlobalPinInfo, common.StreamChannelSize)
		count := 0
		iter = func() (interface{}, bool, error) {
			for {
				select {
				case <-ctx.Done():
					return nil, false, ctx.Err()
				case p, ok := <-out:
					if !ok {
						logger.Infof("statusAllHandler: Finished streaming %d items", count)
						return nil, false, nil
					}
					// Normalize metadata to ensure JSON serialization works
					if p.Metadata != nil {
						p.Metadata = normalizeMetadata(p.Metadata)
					}
					// Apply metadata filter on API side using pre-parsed filter
					if hasMetadataFilter {
						sf := types.StatusFilter{Metadata: metadataFilter}
						matched := sf.MatchMetadata(p.Metadata)
						logger.Debugf("statusAllHandler: CID=%s, matched=%v", p.Cid, matched)
						if !matched {
							continue
						}
					}
					count++
					if count%10 == 0 || count <= 5 {
						logger.Debugf("statusAllHandler: Streamed %d items, latest CID=%s", count, p.Cid)
					}
					return p, true, nil
				}
			}
		}
		go func() {
			defer close(errCh)

			errCh <- api.rpcClient.Stream(
				r.Context(),
				"",
				"Cluster",
				"StatusAll",
				in,
				out,
			)
		}()
	}

	api.StreamResponse(w, iter, errCh)
}

// request statuses for multiple CIDs in parallel.
func (api *API) statusCidsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	queryValues := r.URL.Query()
	filterCidsStr := strings.Split(queryValues.Get("cids"), ",")
	var cids []types.Cid

	for _, cidStr := range filterCidsStr {
		c, err := types.DecodeCid(cidStr)
		if err != nil {
			api.SendResponse(w, http.StatusBadRequest, fmt.Errorf("error decoding Cid: %w", err), nil)
			return
		}
		cids = append(cids, c)
	}

	local := queryValues.Get("local")

	gpiCh := make(chan types.GlobalPinInfo, len(cids))
	errCh := make(chan error, len(cids))
	var wg sync.WaitGroup
	wg.Add(len(cids))

	// Close channel when done
	go func() {
		wg.Wait()
		close(errCh)
		close(gpiCh)
	}()

	if local == "true" {
		for _, ci := range cids {
			go func(c types.Cid) {
				defer wg.Done()
				var pinInfo types.PinInfo
				err := api.rpcClient.CallContext(
					ctx,
					"",
					"Cluster",
					"StatusLocal",
					c,
					&pinInfo,
				)
				if err != nil {
					errCh <- err
					return
				}
				gpiCh <- pinInfo.ToGlobal()
			}(ci)
		}
	} else {
		for _, ci := range cids {
			go func(c types.Cid) {
				defer wg.Done()
				var pinInfo types.GlobalPinInfo
				err := api.rpcClient.CallContext(
					ctx,
					"",
					"Cluster",
					"Status",
					c,
					&pinInfo,
				)
				if err != nil {
					errCh <- err
					return
				}
				gpiCh <- pinInfo
			}(ci)
		}
	}

	iter := func() (interface{}, bool, error) {
		gpi, ok := <-gpiCh
		if !ok {
			return nil, false, nil
		}
		// Normalize metadata so JSON encoding works (RPC may return map[interface{}]interface{})
		if gpi.Metadata != nil {
			gpi.Metadata = normalizeMetadata(gpi.Metadata)
		}
		return gpi, true, nil
	}

	api.StreamResponse(w, iter, errCh)
}

func (api *API) statusHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	if pin := api.ParseCidOrFail(w, r); pin.Defined() {
		if local == "true" {
			var pinInfo types.PinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"StatusLocal",
				pin.Cid,
				&pinInfo,
			)
			// Normalize metadata to ensure JSON serialization works
			if pinInfo.Metadata != nil {
				pinInfo.Metadata = normalizeMetadata(pinInfo.Metadata)
			}
			gpi := pinInfo.ToGlobal()
			if gpi.Metadata != nil {
				gpi.Metadata = normalizeMetadata(gpi.Metadata)
			}
			api.SendResponse(w, common.SetStatusAutomatically, err, gpi)
		} else {
			var pinInfo types.GlobalPinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"Status",
				pin.Cid,
				&pinInfo,
			)
			// Normalize metadata to ensure JSON serialization works
			if pinInfo.Metadata != nil {
				pinInfo.Metadata = normalizeMetadata(pinInfo.Metadata)
			}
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo)
		}
	}
}

func (api *API) recoverAllHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	var iter common.StreamIterator
	in := make(chan struct{})
	close(in)
	errCh := make(chan error, 1)

	if local == "true" {
		out := make(chan types.PinInfo, common.StreamChannelSize)
		iter = func() (interface{}, bool, error) {
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case p, ok := <-out:
				return p.ToGlobal(), ok, nil
			}
		}

		go func() {
			defer close(errCh)

			errCh <- api.rpcClient.Stream(
				r.Context(),
				"",
				"Cluster",
				"RecoverAllLocal",
				in,
				out,
			)
		}()

	} else {
		out := make(chan types.GlobalPinInfo, common.StreamChannelSize)
		iter = func() (interface{}, bool, error) {
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case p, ok := <-out:
				return p, ok, nil
			}
		}
		go func() {
			defer close(errCh)

			errCh <- api.rpcClient.Stream(
				r.Context(),
				"",
				"Cluster",
				"RecoverAll",
				in,
				out,
			)
		}()
	}

	api.StreamResponse(w, iter, errCh)
}

func (api *API) recoverHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	if pin := api.ParseCidOrFail(w, r); pin.Defined() {
		if local == "true" {
			var pinInfo types.PinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"RecoverLocal",
				pin.Cid,
				&pinInfo,
			)
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo.ToGlobal())
		} else {
			var pinInfo types.GlobalPinInfo
			err := api.rpcClient.CallContext(
				r.Context(),
				"",
				"Cluster",
				"Recover",
				pin.Cid,
				&pinInfo,
			)
			api.SendResponse(w, common.SetStatusAutomatically, err, pinInfo)
		}
	}
}

func (api *API) repoGCHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	local := queryValues.Get("local")

	if local == "true" {
		var localRepoGC types.RepoGC
		err := api.rpcClient.CallContext(
			r.Context(),
			"",
			"Cluster",
			"RepoGCLocal",
			struct{}{},
			&localRepoGC,
		)

		api.SendResponse(w, common.SetStatusAutomatically, err, repoGCToGlobal(localRepoGC))
		return
	}

	var repoGC types.GlobalRepoGC
	err := api.rpcClient.CallContext(
		r.Context(),
		"",
		"Cluster",
		"RepoGC",
		struct{}{},
		&repoGC,
	)
	api.SendResponse(w, common.SetStatusAutomatically, err, repoGC)
}

func repoGCToGlobal(r types.RepoGC) types.GlobalRepoGC {
	return types.GlobalRepoGC{
		PeerMap: map[string]types.RepoGC{
			r.Peer.String(): r,
		},
	}
}
