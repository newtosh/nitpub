package store

import bolt "go.etcd.io/bbolt"

const (
	telemetryInstanceIDKey = "telemetry_instance_id"
	telemetryTokenKey      = "telemetry_token"
	telemetryEnabledKey    = "telemetry_enabled"
)

// TelemetryIdentity is the registered instance ID and bearer token issued
// by the (operator-configured, out-of-repo) telemetry receiver.
type TelemetryIdentity struct {
	InstanceID string
	Token      string
}

// GetTelemetryIdentity returns the persisted telemetry identity, if any.
// A zero-value TelemetryIdentity with ok=false means the instance has
// never registered.
func (s *Store) GetTelemetryIdentity() (identity TelemetryIdentity, ok bool, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeta))
		id := b.Get([]byte(telemetryInstanceIDKey))
		token := b.Get([]byte(telemetryTokenKey))
		if id == nil || token == nil {
			return nil
		}
		ok = true
		identity = TelemetryIdentity{InstanceID: string(id), Token: string(token)}
		return nil
	})
	return identity, ok, err
}

// SetTelemetryIdentity persists the instance ID and bearer token returned
// by registration.
func (s *Store) SetTelemetryIdentity(instanceID, token string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeta))
		if err := b.Put([]byte(telemetryInstanceIDKey), []byte(instanceID)); err != nil {
			return err
		}
		return b.Put([]byte(telemetryTokenKey), []byte(token))
	})
}

// TelemetryEnabled reports whether the operator has opted in. Defaults to
// false (the key is absent) so telemetry is inert until explicitly enabled.
func (s *Store) TelemetryEnabled() (bool, error) {
	var enabled bool
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bucketMeta)).Get([]byte(telemetryEnabledKey))
		enabled = len(v) > 0
		return nil
	})
	return enabled, err
}

// SetTelemetryEnabled flips the runtime opt-in flag. This is checked at
// each heartbeat tick, so disabling it stops future sends without a
// restart.
func (s *Store) SetTelemetryEnabled(enabled bool) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeta))
		if enabled {
			return b.Put([]byte(telemetryEnabledKey), []byte("1"))
		}
		return b.Delete([]byte(telemetryEnabledKey))
	})
}
