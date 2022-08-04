// Copyright (c) 2022 ScyllaDB.

package v1alpha1_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	validationv1alpha1 "github.com/scylladb/scylla-operator/pkg/api/validation/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestValidateScyllaDatacenter(t *testing.T) {
	validCluster := unit.NewSingleRackDatacenter(3)
	validCluster.Spec.Datacenter.Racks[0].ScyllaContainer.Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	sameName := validCluster.DeepCopy()
	sameName.Spec.Datacenter.Racks = append(sameName.Spec.Datacenter.Racks, sameName.Spec.Datacenter.Racks[0])

	invalidIntensity := validCluster.DeepCopy()
	invalidIntensity.Spec.Repairs = append(invalidIntensity.Spec.Repairs, scyllav1alpha1.RepairTaskSpec{
		Intensity: "100Mib",
	})

	nonUniqueManagerTaskNames := validCluster.DeepCopy()
	nonUniqueManagerTaskNames.Spec.Backups = append(nonUniqueManagerTaskNames.Spec.Backups, scyllav1alpha1.BackupTaskSpec{
		SchedulerTaskSpec: scyllav1alpha1.SchedulerTaskSpec{
			Name: "task-name",
		},
	})
	nonUniqueManagerTaskNames.Spec.Repairs = append(nonUniqueManagerTaskNames.Spec.Repairs, scyllav1alpha1.RepairTaskSpec{
		SchedulerTaskSpec: scyllav1alpha1.SchedulerTaskSpec{
			Name: "task-name",
		},
	})

	tests := []struct {
		name                string
		obj                 *scyllav1alpha1.ScyllaDatacenter
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid",
			obj:                 validCluster,
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "two racks with same name",
			obj:  sameName,
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenter.racks[1].name", BadValue: "test-rack"},
			},
			expectedErrorString: `spec.datacenter.racks[1].name: Duplicate value: "test-rack"`,
		},
		{
			name: "invalid intensity in repair task spec",
			obj:  invalidIntensity,
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "100Mib", Detail: "invalid intensity, it must be a float value"},
			},
			expectedErrorString: `spec.repairs[0].intensity: Invalid value: "100Mib": invalid intensity, it must be a float value`,
		},
		{
			name: "invalid intensity in repair task spec && non-unique names in manager tasks spec",
			obj:  nonUniqueManagerTaskNames,
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "", Detail: "invalid intensity, it must be a float value"},
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.backups[0].name", BadValue: "task-name"},
			},
			expectedErrorString: `[spec.repairs[0].intensity: Invalid value: "": invalid intensity, it must be a float value, spec.backups[0].name: Duplicate value: "task-name"]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validationv1alpha1.ValidateScyllaDatacenter(test.obj)
			if !reflect.DeepEqual(errList, test.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(test.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, test.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(test.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaDatacenterUpdate(t *testing.T) {
	tests := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDatacenter
		new                 *scyllav1alpha1.ScyllaDatacenter
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewSingleRackDatacenter(3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "major version changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:3.3.1", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "minor version changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:2.4.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "patch version changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "repo changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "new-repo:2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "dcName changed",
			old:  unit.NewSingleRackDatacenter(3),
			new:  unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:2.3.1", "new-dc", "test-rack", 3),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.name", BadValue: "", Detail: "change of datacenter name is currently not supported"},
			},
			expectedErrorString: "spec.datacenter.name: Forbidden: change of datacenter name is currently not supported",
		},
		{
			name:                "rackPlacement changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 placementChanged(unit.NewSingleRackDatacenter(3)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "rackStorage changed",
			old:  unit.NewSingleRackDatacenter(3),
			new:  storageChanged(unit.NewSingleRackDatacenter(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0].storage", BadValue: "", Detail: "changes in storage are currently not supported"},
			},
			expectedErrorString: "spec.datacenter.racks[0].storage: Forbidden: changes in storage are currently not supported",
		},
		{
			name:                "rackResources changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 resourceChanged(unit.NewSingleRackDatacenter(3)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "empty rack removed",
			old:                 unit.NewSingleRackDatacenter(0),
			new:                 racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "empty rack with members under decommission",
			old:  withStatus(unit.NewSingleRackDatacenter(0), scyllav1alpha1.ScyllaDatacenterStatus{Racks: map[string]scyllav1alpha1.RackStatus{"test-rack": {Members: pointer.Int32(3)}}}),
			new:  racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0]", BadValue: "", Detail: `rack "test-rack" can't be removed because the members are being scaled down`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Forbidden: rack "test-rack" can't be removed because the members are being scaled down`,
		},
		{
			name: "empty rack with stale status",
			old:  withStatus(unit.NewSingleRackDatacenter(0), scyllav1alpha1.ScyllaDatacenterStatus{Racks: map[string]scyllav1alpha1.RackStatus{"test-rack": {Stale: pointer.Bool(true), Members: pointer.Int32(0)}}}),
			new:  racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenter.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "empty rack with not reconciled generation",
			old:  withStatus(withGeneration(unit.NewSingleRackDatacenter(0), 123), scyllav1alpha1.ScyllaDatacenterStatus{ObservedGeneration: pointer.Int64(321), Racks: map[string]scyllav1alpha1.RackStatus{"test-rack": {Members: pointer.Int32(3)}}}),
			new:  racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenter.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "non-empty racks deleted",
			old:  unit.NewMultiRackDatacenter(3, 2, 1, 0),
			new:  racksDeleted(unit.NewSingleRackDatacenter(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0]", BadValue: "", Detail: `rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[1]", BadValue: "", Detail: `rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[2]", BadValue: "", Detail: `rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first`},
			},
			expectedErrorString: `[spec.datacenter.racks[0]: Forbidden: rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first, spec.datacenter.racks[1]: Forbidden: rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first, spec.datacenter.racks[2]: Forbidden: rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validationv1alpha1.ValidateScyllaDatacenterUpdate(test.new, test.old)
			if !reflect.DeepEqual(errList, test.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(test.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, test.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(test.expectedErrorString, errStr))
			}
		})
	}
}

func withGeneration(sc *scyllav1alpha1.ScyllaDatacenter, generation int64) *scyllav1alpha1.ScyllaDatacenter {
	sc.Generation = generation
	return sc
}

func withStatus(sc *scyllav1alpha1.ScyllaDatacenter, status scyllav1alpha1.ScyllaDatacenterStatus) *scyllav1alpha1.ScyllaDatacenter {
	sc.Status = status
	return sc
}

func placementChanged(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Datacenter.Racks[0].Placement = &scyllav1alpha1.PlacementSpec{}
	return c
}

func resourceChanged(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Datacenter.Racks[0].ScyllaContainer.Resources.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
	}
	return c
}

func racksDeleted(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Datacenter.Racks = nil
	return c
}

func storageChanged(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Datacenter.Racks[0].Storage.Capacity = "15Gi"
	return c
}
