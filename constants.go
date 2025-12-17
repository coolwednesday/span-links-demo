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
	DefaultBatchSize     = 10
	DefaultWorkerCount   = 2
	BatchPublishInterval = 2 * time.Second
)
