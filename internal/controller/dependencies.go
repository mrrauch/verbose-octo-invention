package controller

import (
	"fmt"
	"strings"
)

func controlPlanePrefix(instanceName, serviceSuffix string) string {
	if !strings.HasSuffix(instanceName, serviceSuffix) {
		return ""
	}
	prefix := strings.TrimSuffix(instanceName, serviceSuffix)
	if prefix == "" {
		return ""
	}
	return prefix
}

func databaseDependency(instanceName, serviceSuffix, namespace string) (host, rootSecret string) {
	databaseName := "database"
	if prefix := controlPlanePrefix(instanceName, serviceSuffix); prefix != "" {
		databaseName = fmt.Sprintf("%s-database", prefix)
	}

	return fmt.Sprintf("%s.%s.svc", databaseName, namespace), fmt.Sprintf("%s-root-password", databaseName)
}

func keystoneDependency(instanceName, serviceSuffix, namespace string) (url, adminSecret string) {
	keystoneName := "keystone"
	if prefix := controlPlanePrefix(instanceName, serviceSuffix); prefix != "" {
		keystoneName = fmt.Sprintf("%s-keystone", prefix)
	}

	return fmt.Sprintf("http://%s-api.%s.svc:5000/v3", keystoneName, namespace), fmt.Sprintf("%s-admin-password", keystoneName)
}

func ovnDependency(instanceName, serviceSuffix, namespace string) (northboundDB, southboundDB string) {
	nbDBName := "ovn-nb-db"
	sbDBName := "ovn-sb-db"
	if prefix := controlPlanePrefix(instanceName, serviceSuffix); prefix != "" {
		ovnName := fmt.Sprintf("%s-ovn", prefix)
		nbDBName = fmt.Sprintf("%s-nb-db", ovnName)
		sbDBName = fmt.Sprintf("%s-sb-db", ovnName)
	}

	return fmt.Sprintf("tcp:%s.%s.svc:6641", nbDBName, namespace), fmt.Sprintf("tcp:%s.%s.svc:6642", sbDBName, namespace)
}
