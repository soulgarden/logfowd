package dictionary

const (
	DumpFilePermissions     = 0o600
	FlushChangesNumber      = 1024
	FlushChangesChannelSize = FlushChangesNumber * 2
	FlushLogsNumber         = 1024
)

// nolint: lll
const K8sContainerRegexp = `^/var/log/containers/(?P<pod_name>[a-z0-9-]+)_(?P<namespace>[a-z-]+)_(?P<container_name>[a-z-]+)-(?P<container_id>[a-z-0-9]{64}).log$`

// nolint: lll
const K8sPodsRegexp = `^/var/log/pods/(?P<namespace>[a-z0-9-]+)_(?P<pod_name>[a-z0-9-]+)_(?P<pod_id>[a-z0-9-]+)/(?P<container_name>[a-z-0-9]+)/(?P<num>[0-9]+).log$`
