package controller

import (
	"fmt"

	openstackv1alpha1 "github.com/mrrauch/openstack-operator/api/v1alpha1"
)

func databaseEngineOrDefault(engine openstackv1alpha1.DatabaseEngine) openstackv1alpha1.DatabaseEngine {
	switch engine {
	case openstackv1alpha1.DatabaseEngineMySQL, openstackv1alpha1.DatabaseEngineMariaDB, openstackv1alpha1.DatabaseEnginePostgreSQL:
		return engine
	default:
		return openstackv1alpha1.DatabaseEnginePostgreSQL
	}
}

func serviceDatabaseSecretName(instanceName string, dbConfig openstackv1alpha1.DatabaseConfig) string {
	if dbConfig.SecretName != "" {
		return dbConfig.SecretName
	}
	return fmt.Sprintf("%s-db-password", instanceName)
}
