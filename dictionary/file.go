package dictionary

const (
	FlushLogsNumber = 1024
)

// nolint: lll
const K8sPodsRegexp = `^/var/log/pods/(?P<namespace>[a-z0-9-]+)_(?P<pod_name>[a-z0-9-]+)_(?P<pod_id>[a-z0-9-]+)/(?P<container_name>[a-z-0-9]+)/(?P<num>[0-9]+).log$`
