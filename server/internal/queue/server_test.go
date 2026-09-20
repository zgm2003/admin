package queue

import "testing"

func TestWorkerQueueConfigConsumesDefaultAndMaintenance(t *testing.T) {
	config := workerQueueConfig()

	if config["default"] <= 0 {
		t.Fatalf("default queue priority = %d, want positive", config["default"])
	}
	if config["maintenance"] <= 0 {
		t.Fatalf("maintenance queue priority = %d, want positive", config["maintenance"])
	}
}
