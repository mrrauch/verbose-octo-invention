package images

// Default container images for OpenStack services.
// All images are from the Kolla project for the 2025.1 (Epoxy) release.
const (
	DefaultMariaDB       = "quay.io/openstack.kolla/mariadb-server:2025.1"
	DefaultRabbitMQ      = "quay.io/openstack.kolla/rabbitmq:2025.1"
	DefaultMemcached     = "quay.io/openstack.kolla/memcached:2025.1"
	DefaultKeystone      = "quay.io/openstack.kolla/keystone:2025.1"
	DefaultGlanceAPI     = "quay.io/openstack.kolla/glance-api:2025.1"
	DefaultPlacement     = "quay.io/openstack.kolla/placement-api:2025.1"
	DefaultNeutronServer = "quay.io/openstack.kolla/neutron-server:2025.1"
	DefaultNovaAPI       = "quay.io/openstack.kolla/nova-api:2025.1"
	DefaultNovaScheduler = "quay.io/openstack.kolla/nova-scheduler:2025.1"
	DefaultNovaConductor = "quay.io/openstack.kolla/nova-conductor:2025.1"
	DefaultNovaCompute   = "quay.io/openstack.kolla/nova-compute:2025.1"
	DefaultOVNNorthd     = "quay.io/openstack.kolla/ovn-northd:2025.1"
	DefaultOVNNBDB       = "quay.io/openstack.kolla/ovn-nb-db-server:2025.1"
	DefaultOVNSBDB       = "quay.io/openstack.kolla/ovn-sb-db-server:2025.1"
)

// ImageOrDefault returns the image if non-empty, otherwise the defaultImage.
func ImageOrDefault(image, defaultImage string) string {
	if image != "" {
		return image
	}
	return defaultImage
}
