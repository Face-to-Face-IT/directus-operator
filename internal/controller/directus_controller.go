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

package controller

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/zjpiazza/directus-operator/api/v1alpha1"
)

// DirectusReconciler reconciles a Directus object
type DirectusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.directus.io,resources=directuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.directus.io,resources=directuses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.directus.io,resources=directuses/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DirectusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Directus instance
	directus := &appsv1alpha1.Directus{}
	err := r.Get(ctx, req.NamespacedName, directus)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Reconcile PVC for SQLite if needed
	if directus.Spec.Database.Client == "sqlite3" && directus.Spec.Database.SQLite != nil && directus.Spec.Database.SQLite.Persistence != nil {
		err = r.reconcilePVC(ctx, directus)
		if err != nil {
			log.Error(err, "Failed to reconcile PVC")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Secrets
	err = r.reconcileSecrets(ctx, directus)
	if err != nil {
		log.Error(err, "Failed to reconcile Secrets")
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	err = r.reconcileDeployment(ctx, directus)
	if err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	// Reconcile Service
	err = r.reconcileService(ctx, directus)
	if err != nil {
		log.Error(err, "Failed to reconcile Service")
		return ctrl.Result{}, err
	}

	// Reconcile Ingress
	if directus.Spec.Ingress.Enabled {
		err = r.reconcileIngress(ctx, directus)
		if err != nil {
			log.Error(err, "Failed to reconcile Ingress")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DirectusReconciler) reconcilePVC(ctx context.Context, directus *appsv1alpha1.Directus) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      directus.Name + "-data",
			Namespace: directus.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}

		storage := directus.Spec.Database.SQLite.Persistence.Size
		if storage == "" {
			storage = "1Gi"
		}

		qty, err := resource.ParseQuantity(storage)
		if err != nil {
			return err
		}

		pvc.Spec.Resources = corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: qty,
			},
		}

		if directus.Spec.Database.SQLite.Persistence.StorageClassName != "" {
			pvc.Spec.StorageClassName = &directus.Spec.Database.SQLite.Persistence.StorageClassName
		}

		return ctrl.SetControllerReference(directus, pvc, r.Scheme)
	})

	return err
}

func (r *DirectusReconciler) reconcileSecrets(ctx context.Context, directus *appsv1alpha1.Directus) error {
	secretName := directus.Name + "-secrets"
	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: directus.Namespace}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new secret
			key := string(uuid.NewUUID())
			sec := string(uuid.NewUUID())
			adminPassword := string(uuid.NewUUID()) // Simple random string
			adminEmail := "admin@example.com"
			if directus.Spec.AdminEmail != "" {
				adminEmail = directus.Spec.AdminEmail
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: directus.Namespace,
				},
				StringData: map[string]string{
					"KEY":            key,
					"SECRET":         sec,
					"ADMIN_EMAIL":    adminEmail,
					"ADMIN_PASSWORD": adminPassword,
				},
			}
			// Set owner ref
			if err := ctrl.SetControllerReference(directus, secret, r.Scheme); err != nil {
				return err
			}
			return r.Create(ctx, secret)
		}
		return err
	}
	return nil
}

func (r *DirectusReconciler) reconcileDeployment(ctx context.Context, directus *appsv1alpha1.Directus) error {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      directus.Name,
			Namespace: directus.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, dep, func() error {
		replicas := int32(1)
		if directus.Spec.Replicas != nil {
			replicas = *directus.Spec.Replicas
		}
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": directus.Name},
		}
		dep.Spec.Template.ObjectMeta.Labels = map[string]string{"app": directus.Name}

		// Apply pod annotations if specified
		if len(directus.Spec.PodAnnotations) > 0 {
			if dep.Spec.Template.ObjectMeta.Annotations == nil {
				dep.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
			}
			for k, v := range directus.Spec.PodAnnotations {
				dep.Spec.Template.ObjectMeta.Annotations[k] = v
			}
		}

		// Init Container for Extensions
		initContainers := []corev1.Container{}
		volumeMounts := []corev1.VolumeMount{}
		volumes := []corev1.Volume{}

		if len(directus.Spec.Extensions) > 0 {
			// Create a volume for extensions
			volumes = append(volumes, corev1.Volume{
				Name: "extensions",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      "extensions",
				MountPath: "/directus/extensions",
			})

			// Init container to download extensions
			// We use node:20-alpine to support npm installs and git
			cmd := "apk add --no-cache git && mkdir -p /temp-extensions && cd /temp-extensions && "

			initContainerEnv := []corev1.EnvVar{}
			initContainerVolumeMounts := []corev1.VolumeMount{
				{
					Name:      "extensions",
					MountPath: "/directus/extensions",
				},
			}

			if directus.Spec.ExtensionsConfig != nil && directus.Spec.ExtensionsConfig.SecretRef != "" {
				volumes = append(volumes, corev1.Volume{
					Name: "npmrc",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: directus.Spec.ExtensionsConfig.SecretRef,
						},
					},
				})
				initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
					Name:      "npmrc",
					MountPath: "/root/.npmrc",
					SubPath:   ".npmrc",
				})
			}

			for _, ext := range directus.Spec.Extensions {
				cmd += fmt.Sprintf("echo 'Processing extension %s'; ", ext.Name)

				extType := ext.Type
				source := ext.Source

				if strings.HasPrefix(source, "npm:") {
					extType = "npm"
					source = strings.TrimPrefix(source, "npm:")
				}

				if extType == "npm" {
					// Install via npm
					installCmd := source
					if ext.Version != "" {
						installCmd = fmt.Sprintf("%s@%s", source, ext.Version)
					}
					cmd += fmt.Sprintf("npm install %s && ", installCmd)

					// Extract package name from source to handle versions (e.g. pkg@1.0.0)
					pkgName := source
					if strings.HasPrefix(pkgName, "@") {
						if idx := strings.Index(pkgName[1:], "@"); idx != -1 {
							pkgName = pkgName[:idx+1]
						}
					} else {
						if idx := strings.Index(pkgName, "@"); idx != -1 {
							pkgName = pkgName[:idx]
						}
					}

					// Move from node_modules to the target directory
					// We assume Source is the package name.
					cmd += fmt.Sprintf("mkdir -p /directus/extensions/%s && cp -r node_modules/%s/* /directus/extensions/%s/; ", ext.Name, pkgName, ext.Name)
				} else if extType == "git" {
					// Git clone
					cmd += fmt.Sprintf("mkdir -p /directus/extensions/%s && ", ext.Name)
					cmd += fmt.Sprintf("git clone %s /directus/extensions/%s; ", source, ext.Name)
				} else {
					// Default to URL/tarball
					cmd += fmt.Sprintf("mkdir -p /directus/extensions/%s && ", ext.Name)
					cmd += fmt.Sprintf("wget -O /temp-extensions/%s.tar.gz %s && tar -xzf /temp-extensions/%s.tar.gz -C /directus/extensions/%s; ", ext.Name, source, ext.Name, ext.Name)
				}
			}

			initContainers = append(initContainers, corev1.Container{
				Name:         "install-extensions",
				Image:        "node:20-alpine",
				Command:      []string{"sh", "-c", cmd},
				VolumeMounts: initContainerVolumeMounts,
				Env:          initContainerEnv,
			})
		}

		dep.Spec.Template.Spec.InitContainers = initContainers
		dep.Spec.Template.Spec.Volumes = volumes

		// Main Container
		container := corev1.Container{
			Name:  "directus",
			Image: directus.Spec.Image,
			Ports: []corev1.ContainerPort{{ContainerPort: 8055}},
			Env: []corev1.EnvVar{
				{
					Name: "KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: directus.Name + "-secrets"},
							Key:                  "KEY",
						},
					},
				},
				{
					Name: "SECRET",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: directus.Name + "-secrets"},
							Key:                  "SECRET",
						},
					},
				},
				{
					Name: "ADMIN_EMAIL",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: directus.Name + "-secrets"},
							Key:                  "ADMIN_EMAIL",
						},
					},
				},
				{
					Name: "ADMIN_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: directus.Name + "-secrets"},
							Key:                  "ADMIN_PASSWORD",
						},
					},
				},
			},
			VolumeMounts: volumeMounts,
		}

		if directus.Spec.PublicURL != "" {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "PUBLIC_URL",
				Value: directus.Spec.PublicURL,
			})
		} else if directus.Spec.Ingress.Enabled && directus.Spec.Ingress.Host != "" {
			protocol := "http"
			if directus.Spec.Ingress.TLS {
				protocol = "https"
			}
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "PUBLIC_URL",
				Value: fmt.Sprintf("%s://%s", protocol, directus.Spec.Ingress.Host),
			})
		}

		// Database Configuration
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "DB_CLIENT",
			Value: directus.Spec.Database.Client,
		})

		if directus.Spec.Database.Client == "sqlite3" {
			if directus.Spec.Database.SQLite != nil {
				container.Env = append(container.Env, corev1.EnvVar{
					Name:  "DB_FILENAME",
					Value: directus.Spec.Database.SQLite.Filename,
				})

				if directus.Spec.Database.SQLite.Persistence != nil {
					// Mount PVC
					dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: directus.Name + "-data",
							},
						},
					})
					container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
						Name:      "data",
						MountPath: "/directus/database", // Assuming filename is relative or in this dir
					})
				}
			}
		} else {
			// Networked Database
			if directus.Spec.Database.Connection != nil {
				conn := directus.Spec.Database.Connection

				// Check if ConnectionSecretRef is set - use secret references for all values
				if conn.ConnectionSecretRef != nil {
					secretRef := conn.ConnectionSecretRef

					// Host from secret
					hostKey := "host"
					if secretRef.HostKey != "" {
						hostKey = secretRef.HostKey
					}
					container.Env = append(container.Env, corev1.EnvVar{
						Name: "DB_HOST",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  hostKey,
							},
						},
					})

					// Port from secret
					portKey := "port"
					if secretRef.PortKey != "" {
						portKey = secretRef.PortKey
					}
					container.Env = append(container.Env, corev1.EnvVar{
						Name: "DB_PORT",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  portKey,
							},
						},
					})

					// Database name from secret
					dbKey := "dbname"
					if secretRef.DatabaseKey != "" {
						dbKey = secretRef.DatabaseKey
					}
					container.Env = append(container.Env, corev1.EnvVar{
						Name: "DB_DATABASE",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  dbKey,
							},
						},
					})

					// User from secret
					userKey := "user"
					if secretRef.UserKey != "" {
						userKey = secretRef.UserKey
					}
					container.Env = append(container.Env, corev1.EnvVar{
						Name: "DB_USER",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  userKey,
							},
						},
					})

					// Password from secret
					passwordKey := "password"
					if secretRef.PasswordKey != "" {
						passwordKey = secretRef.PasswordKey
					}
					container.Env = append(container.Env, corev1.EnvVar{
						Name: "DB_PASSWORD",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
								Key:                  passwordKey,
							},
						},
					})
				} else {
					// Use individual field values (existing behavior)
					if conn.Host != "" {
						container.Env = append(container.Env, corev1.EnvVar{Name: "DB_HOST", Value: conn.Host})
					}
					if conn.Port != 0 {
						container.Env = append(container.Env, corev1.EnvVar{Name: "DB_PORT", Value: fmt.Sprintf("%d", conn.Port)})
					}
					if conn.Database != "" {
						container.Env = append(container.Env, corev1.EnvVar{Name: "DB_DATABASE", Value: conn.Database})
					}
					if conn.User != "" {
						container.Env = append(container.Env, corev1.EnvVar{Name: "DB_USER", Value: conn.User})
					}

					if conn.PasswordSecretRef != nil {
						container.Env = append(container.Env, corev1.EnvVar{
							Name: "DB_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: conn.PasswordSecretRef.Name},
									Key:                  conn.PasswordSecretRef.Key,
								},
							},
						})
					}
				}

				if conn.ConnectString != "" {
					container.Env = append(container.Env, corev1.EnvVar{Name: "DB_CONNECT_STRING", Value: conn.ConnectString})
				}

				if conn.SSL != nil {
					if conn.SSL.Mode != "" {
						container.Env = append(container.Env, corev1.EnvVar{Name: "DB_SSL", Value: "true"})
						// Note: Directus might need more specific SSL env vars depending on the driver
					}
				}
			}
		}

		dep.Spec.Template.Spec.Containers = []corev1.Container{container}

		// Apply resources if specified
		if directus.Spec.Resources.Limits != nil || directus.Spec.Resources.Requests != nil {
			dep.Spec.Template.Spec.Containers[0].Resources = directus.Spec.Resources
		}

		return ctrl.SetControllerReference(directus, dep, r.Scheme)
	})

	return err
}

func (r *DirectusReconciler) reconcileService(ctx context.Context, directus *appsv1alpha1.Directus) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      directus.Name,
			Namespace: directus.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Selector = map[string]string{"app": directus.Name}
		svc.Spec.Ports = []corev1.ServicePort{
			{
				Port:       8055,
				TargetPort: intstr.FromInt(8055),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		return ctrl.SetControllerReference(directus, svc, r.Scheme)
	})

	return err
}

func (r *DirectusReconciler) reconcileIngress(ctx context.Context, directus *appsv1alpha1.Directus) error {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      directus.Name,
			Namespace: directus.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, ing, func() error {
		pathType := networkingv1.PathTypePrefix
		ing.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: directus.Spec.Ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &pathType,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: directus.Name,
										Port: networkingv1.ServiceBackendPort{
											Number: 8055,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		return ctrl.SetControllerReference(directus, ing, r.Scheme)
	})

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *DirectusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Directus{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
