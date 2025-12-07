package main

import "time"

// Processing timeouts for order processing steps
const (
	ValidationTimeout = 100 * time.Millisecond
	PaymentTimeout    = 150 * time.Millisecond
	ShippingTimeout   = 120 * time.Millisecond
)

// Queue configuration
const (
	DefaultQueueCapacity = 100
	DefaultBatchSize     = 3
	DefaultWorkerCount   = 2
	BatchPublishInterval = 2 * time.Second
)

// Log message keys for consistent logging
const (
	LogKeyOrderID     = "order.id"
	LogKeyCustomerID  = "customer.id"
	LogKeyWorkerID    = "worker.id"
	LogKeyAmount      = "order.amount"
	LogKeyStatus      = "status"
	LogKeyDuration    = "duration_seconds"
	LogKeyBatchSize   = "batch.size"
	LogKeyError       = "error"
)

