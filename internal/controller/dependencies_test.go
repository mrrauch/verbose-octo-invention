package controller

import "testing"

func TestDatabaseDependency_ControlPlaneManaged(t *testing.T) {
	host, secret := databaseDependency("my-cloud-keystone", "-keystone", "openstack")
	if host != "my-cloud-database.openstack.svc" {
		t.Fatalf("expected host my-cloud-database.openstack.svc, got %s", host)
	}
	if secret != "my-cloud-database-root-password" {
		t.Fatalf("expected secret my-cloud-database-root-password, got %s", secret)
	}
}

func TestDatabaseDependency_Standalone(t *testing.T) {
	host, secret := databaseDependency("keystone", "-keystone", "openstack")
	if host != "database.openstack.svc" {
		t.Fatalf("expected host database.openstack.svc, got %s", host)
	}
	if secret != "database-root-password" {
		t.Fatalf("expected secret database-root-password, got %s", secret)
	}
}

func TestKeystoneDependency_ControlPlaneManaged(t *testing.T) {
	url, secret := keystoneDependency("my-cloud-glance", "-glance", "openstack")
	if url != "http://my-cloud-keystone-api.openstack.svc:5000/v3" {
		t.Fatalf("expected url http://my-cloud-keystone-api.openstack.svc:5000/v3, got %s", url)
	}
	if secret != "my-cloud-keystone-admin-password" {
		t.Fatalf("expected secret my-cloud-keystone-admin-password, got %s", secret)
	}
}

func TestOVNDependency_ControlPlaneManaged(t *testing.T) {
	nbDB, sbDB := ovnDependency("my-cloud-neutron", "-neutron", "openstack")
	if nbDB != "tcp:my-cloud-ovn-nb-db.openstack.svc:6641" {
		t.Fatalf("expected nb db endpoint tcp:my-cloud-ovn-nb-db.openstack.svc:6641, got %s", nbDB)
	}
	if sbDB != "tcp:my-cloud-ovn-sb-db.openstack.svc:6642" {
		t.Fatalf("expected sb db endpoint tcp:my-cloud-ovn-sb-db.openstack.svc:6642, got %s", sbDB)
	}
}
