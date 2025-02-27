/*
Copyright 2025 Senthil Kumaran.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	networkingv1 "github.com/orsenthil/kubernetes-ip-tracker/api/v1"
)

var _ = Describe("PodTracker Controller", func() {
	Context("When creating a PodTracker", func() {
		It("Should create successfully", func() {
			ctx := context.Background()

			podTracker := &networkingv1.PodTracker{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "networking.learntosolveit.com/v1",
					Kind:       "PodTracker",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-podtracker",
				},
				Spec: networkingv1.PodTrackerSpec{
					Namespace: "",
				},
			}

			Expect(k8sClient.Create(ctx, podTracker)).Should(Succeed())

			// Verify it was created
			fetched := &networkingv1.PodTracker{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: "test-podtracker"}, fetched)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			// Cleanup
			Expect(k8sClient.Delete(ctx, podTracker)).Should(Succeed())
		})
	})
})
