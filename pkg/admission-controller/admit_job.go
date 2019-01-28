/*
Copyright 2018 The Volcano Authors.

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

package admission

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	v1alpha1 "hpw.cloud/volcano/pkg/apis/batch/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
)

// job admit.
func AdmitJobs(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	glog.V(3).Infof("admitting jobs")
	jobResource := metav1.GroupVersionResource{Group: v1alpha1.SchemeGroupVersion.Group, Version: v1alpha1.SchemeGroupVersion.Version, Resource: "jobs"}
	if ar.Request.Resource != jobResource {
		err := fmt.Errorf("expect resource to be %s", jobResource)
		return ToAdmissionResponse(err)
	}

	if ar.Request.Operation != "CREATE" && ar.Request.Operation != "UPDATE" {
		err := fmt.Errorf("expect operation to be 'CREATE' or 'UPDATE'")
		return ToAdmissionResponse(err)
	}

	raw := ar.Request.Object.Raw
	job := v1alpha1.Job{}

	deserializer := Codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &job); err != nil {
		return ToAdmissionResponse(err)
	}

	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	glog.V(3).Infof("the job struct is %+v", job)

	var msg string

	taskPolicyEvents :=  map[v1alpha1.Event]v1alpha1.Event{}
	jobPolicyEvents :=  map[v1alpha1.Event]v1alpha1.Event{}
	taskNames := map[string]string{}

	var totalReplicas int32
	totalReplicas = 0

	minAvailable := job.Spec.MinAvailable
	for _, task := range job.Spec.Tasks {

		// count replicas
		totalReplicas = totalReplicas + task.Replicas

		// duplicate task name 
		if _, found := taskNames[task.Name]; found {
			reviewResponse.Allowed = false
			msg = msg + " duplicated task name"
			break
		}else{
			taskNames[task.Name] = task.Name
		}

		//duplicate task policies event
		for _, taskPolicy := range task.Policies {
			if _, found := taskPolicyEvents[taskPolicy.Event]; found{
				reviewResponse.Allowed = false
				msg = msg + " duplicated task policies event"
				break
			}else{
				taskPolicyEvents[taskPolicy.Event] = taskPolicy.Event
			}
		}
	}

	if totalReplicas < minAvailable {
		reviewResponse.Allowed = false;
		msg = msg + " minAvailable should not be larger than total replicas in tasks"
	}

	//duplicate job policies event
	for _, jobPolicy := range job.Spec.Policies {
		if _, found := jobPolicyEvents[jobPolicy.Event]; found{
			reviewResponse.Allowed = false
			msg = msg + " duplicated task policies event"
			break
		}else{
			jobPolicyEvents[jobPolicy.Event] = jobPolicy.Event
		}
	}

	if !reviewResponse.Allowed {
		reviewResponse.Result = &metav1.Status{Message: strings.TrimSpace(msg)}
	}
	return &reviewResponse
}