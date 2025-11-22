/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DirectusSpec defines the desired state of Directus
type DirectusSpec struct {
	// Image is the Directus image to use.
	// +optional
	Image string `json:"image,omitempty"`

	// Replicas is the number of replicas.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Database configures the database connection.
	// +optional
	Database DatabaseConfig `json:"database,omitempty"`

	// Extensions is a list of extensions to install.
	// +optional
	Extensions []Extension `json:"extensions,omitempty"`

	// ExtensionsConfig configures extension installation.
	// +optional
	ExtensionsConfig *ExtensionsConfig `json:"extensionsConfig,omitempty"`

	// Ingress configures the ingress.
	// +optional
	Ingress IngressConfig `json:"ingress,omitempty"`

	// PublicURL overrides the PUBLIC_URL environment variable.
	// +optional
	PublicURL string `json:"publicUrl,omitempty"`

	// AdminEmail is the email for the admin user.
	// +optional
	AdminEmail string `json:"adminEmail,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.client == 'sqlite3' ? has(self.sqlite) : !has(self.sqlite)",message="sqlite config must be set if and only if client is sqlite3"
// +kubebuilder:validation:XValidation:rule="self.client != 'sqlite3' ? has(self.connection) : !has(self.connection)",message="connection config must be set if and only if client is not sqlite3"
type DatabaseConfig struct {
	// Client specifies the database driver to use.
	// +kubebuilder:validation:Enum=pg;mysql;sqlite3;mssql;oracledb;cockroachdb
	Client string `json:"client"`

	// Connection configures a networked database connection.
	// +optional
	Connection *NetworkedDatabaseConfig `json:"connection,omitempty"`

	// SQLite configures an embedded SQLite database.
	// +optional
	SQLite *SQLiteConfig `json:"sqlite,omitempty"`
}

type NetworkedDatabaseConfig struct {
	// Host is the database host.
	// +optional
	Host string `json:"host,omitempty"`

	// Port is the database port.
	// +optional
	Port int `json:"port,omitempty"`

	// Database is the database name.
	// +optional
	Database string `json:"database,omitempty"`

	// User is the database user.
	// +optional
	User string `json:"user,omitempty"`

	// PasswordSecretRef references a secret containing the password.
	// +optional
	PasswordSecretRef *SecretRef `json:"passwordSecretRef,omitempty"`

	// ConnectString is the connection string for Oracle.
	// +optional
	ConnectString string `json:"connectString,omitempty"`

	// SSL configures SSL settings.
	// +optional
	SSL *SSLConfig `json:"ssl,omitempty"`
}

type SQLiteConfig struct {
	// Filename is the path to the SQLite database file.
	Filename string `json:"filename"`

	// Persistence configures the PVC for SQLite.
	// +optional
	Persistence *PersistenceConfig `json:"persistence,omitempty"`
}

type PersistenceConfig struct {
	// Size is the size of the PVC.
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClassName is the storage class for the PVC.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

type SSLConfig struct {
	// Mode is the SSL mode.
	// +kubebuilder:validation:Enum=disable;require;verify-ca;verify-full
	// +optional
	Mode string `json:"mode,omitempty"`

	// CASecretRef references the CA certificate secret.
	// +optional
	CASecretRef *SecretRef `json:"caSecretRef,omitempty"`

	// CertSecretRef references the client certificate secret.
	// +optional
	CertSecretRef *SecretRef `json:"certSecretRef,omitempty"`

	// KeySecretRef references the client key secret.
	// +optional
	KeySecretRef *SecretRef `json:"keySecretRef,omitempty"`
}

type SecretRef struct {
	// Name is the name of the secret.
	Name string `json:"name"`

	// Key is the key in the secret.
	Key string `json:"key"`
}

type Extension struct {
	// Name is the name of the extension.
	Name string `json:"name"`

	// Source is the URL to download the extension from (git or http).
	Source string `json:"source"`

	// Version is the version of the extension to install (npm only).
	// +optional
	Version string `json:"version,omitempty"`

	// Type is the type of extension (interface, layout, etc.).
	// +optional
	Type string `json:"type,omitempty"`
}

type ExtensionsConfig struct {
	// SecretRef is the name of the secret containing the .npmrc file.
	// The secret must have a key named ".npmrc".
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
}

type IngressConfig struct {
	// Enabled enables ingress.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Host is the ingress host.
	// +optional
	Host string `json:"host,omitempty"`

	// TLS enables TLS.
	// +optional
	TLS bool `json:"tls,omitempty"`
}

// DirectusStatus defines the observed state of Directus
type DirectusStatus struct {
	// DatabaseReady indicates if the database is ready.
	DatabaseReady bool `json:"databaseReady,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Directus is the Schema for the directuses API
type Directus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DirectusSpec   `json:"spec,omitempty"`
	Status DirectusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DirectusList contains a list of Directus
type DirectusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Directus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Directus{}, &DirectusList{})
}
