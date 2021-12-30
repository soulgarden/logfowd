package dictionary

const (
	DumpFilePermissions     = 0o600
	FlushChangesNumber      = 1024
	FlushChangesChannelSize = FlushChangesNumber
	FlushLogsNumber         = 1024
	LogsChannelSize         = 32768
)

// nolint: lll
const K8sRegexp = `^/var/log/containers/(?P<pod_name>[a-z0-9-]+)_(?P<namespace>[a-z-]+)_(?P<container_name>[a-z-]+)-(?P<container_id>[a-z-0-9]{64}).log$`
