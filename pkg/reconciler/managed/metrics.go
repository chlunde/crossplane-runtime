/*
Copyright 2022 The Crossplane Authors.

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

package managed

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	managedStatusSynced = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "managed_status_synced",
			Help: "Managed resources is synced",
		},
		[]string{"kind", "name"},
	)
	managedStatusReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "managed_status_ready",
			Help: "Managed resources is ready",
		},
		[]string{"kind", "name"},
	)
	managedStatusDeleting = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "managed_status_deleting",
			Help: "Managed resources is deleting",
		},
		[]string{"kind", "name"},
	)
)

type MetricsReconciler interface {
	ReconcileMetrics(req reconcile.Request, managed resource.Managed)
}

// NewNopMetricsReconciler returns a no-op metrics collector
func NewNopMetricsReconciler() nopMetricsReconciler {
	return nopMetricsReconciler{}
}

type nopMetricsReconciler struct{}

func (n nopMetricsReconciler) ReconcileMetrics(req reconcile.Request, managed resource.Managed) {}

// NewPrometheusMetricsReconciler returns a prometheus metrics
// reconciler. Only one instance should be created as the collectors
// should only be registered once.
func NewPrometheusMetricsReconciler(registry metrics.RegistererGatherer) prometheusMetricsReconciler {
	registry.MustRegister(managedStatusDeleting, managedStatusReady, managedStatusSynced)
	return prometheusMetricsReconciler{}
}

type prometheusMetricsReconciler struct{}

func (p prometheusMetricsReconciler) ReconcileMetrics(req reconcile.Request, managed resource.Managed) {
	kind := managed.GetObjectKind().GroupVersionKind().Kind
	name := managed.GetName()

	ready := managed.GetCondition(xpv1.TypeReady).Status
	synced := managed.GetCondition(xpv1.TypeSynced).Status

	deleting := corev1.ConditionFalse
	if meta.WasDeleted(managed) {
		deleting = corev1.ConditionTrue
	}

	gauges := []struct {
		gauge *prometheus.GaugeVec
		value corev1.ConditionStatus
	}{
		{
			gauge: managedStatusReady,
			value: ready,
		},
		{
			gauge: managedStatusSynced,
			value: synced,
		},
		{
			gauge: managedStatusDeleting,
			value: deleting,
		},
	}

	labelValues := []string{kind, name}

	// attempt to clean up metrics for objects that will cease to
	// exist when the API server will run garbage collection
	if meta.WasDeleted(managed) && len(managed.GetFinalizers()) == 0 {
		for _, g := range gauges {
			g.gauge.DeleteLabelValues(labelValues...)
		}
	} else {
		for _, g := range gauges {
			val := 0.0
			if g.value == corev1.ConditionTrue {
				val = 1
			}

			g.gauge.WithLabelValues(labelValues...).Set(val)
		}
	}
}
