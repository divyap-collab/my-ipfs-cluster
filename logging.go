package ipfscluster

import (
	logging "github.com/ipfs/go-log/v2"
)

var logger = logging.Logger("cluster")

// LoggingFacilities provides a list of logging identifiers
// used by cluster and their default logging level.
//
// Chatty per-request / per-pin facilities default to ERROR so production
// journald/rsyslog are not flooded under upload and metadata PATCH load.
// Override with IPFS_CLUSTER_LOG_LEVEL (e.g. "info" or "error,crdt:info").
var LoggingFacilities = map[string]string{
	"cluster":      "INFO",
	"restapi":      "ERROR",
	"restapilog":   "ERROR",
	"pinsvcapi":    "ERROR",
	"pinsvcapilog": "ERROR",
	"ipfsproxy":    "ERROR",
	"ipfsproxylog": "ERROR",
	"ipfshttp":     "ERROR",
	"monitor":      "INFO",
	"dsstate":      "ERROR",
	"raft":         "INFO",
	"crdt":         "ERROR",
	"pintracker":   "ERROR",
	"diskinfo":     "INFO",
	"tags":         "ERROR",
	"apitypes":     "ERROR",
	"config":       "INFO",
	"shardingdags": "ERROR",
	"singledags":   "ERROR",
	"adder":        "ERROR",
	"optracker":    "ERROR",
	"pstoremgr":    "ERROR",
	"allocator":    "ERROR",
}

// LoggingFacilitiesExtra provides logging identifiers
// used in ipfs-cluster dependencies, which may be useful
// to display. Along with their default value.
var LoggingFacilitiesExtra = map[string]string{
	"p2p-gorpc":   "ERROR",
	"swarm2":      "ERROR",
	"libp2p-raft": "FATAL",
	"raftlib":     "ERROR",
	"badger":      "INFO",
	"badger3":     "INFO",
	"pebble":      "WARN", // pebble logs with INFO and FATAL only
}

// SetFacilityLogLevel sets the log level for a given module
func SetFacilityLogLevel(f, l string) {
	/*
		case "debug", "DEBUG":
			*l = DebugLevel
		case "info", "INFO", "": // make the zero value useful
			*l = InfoLevel
		case "warn", "WARN":
			*l = WarnLevel
		case "error", "ERROR":
			*l = ErrorLevel
		case "dpanic", "DPANIC":
			*l = DPanicLevel
		case "panic", "PANIC":
			*l = PanicLevel
		case "fatal", "FATAL":
			*l = FatalLevel
	*/
	logging.SetLogLevel(f, l)
}
