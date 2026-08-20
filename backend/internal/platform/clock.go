package platform

import "time"

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type FixedClock struct{ Value time.Time }

func (c FixedClock) Now() time.Time { return c.Value }
