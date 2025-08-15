package snmp

import (
	"testing"
	"time"

	cache "github.com/patrickmn/go-cache"
)

const TEST = "Test"

func TestSegment_SNMPInterface_cacheBehaviour(t *testing.T) {
	snmpInterface := &Snmp{}
	segment := snmpInterface.New(map[string]string{"cache_interval": "1ms"})
	if segment == nil {
		t.Error("([error] Segment SNMP did not initiate despite good base config.")
	}
	snmp, ok := segment.(*Snmp)
	if !ok {
		t.Error("([error] Segment SNMP did not initialize correctly.")
	}

	cache := cache.New(snmp.CacheInterval, snmp.CacheInterval)

	//test default timeout
	cache.Add(TEST, TEST, 1*time.Millisecond)
	time.Sleep(1 * time.Millisecond) //item should no longer be in cache
	_, found := cache.Get(TEST)
	if found {
		t.Error("([error] Segment SNMP-Cache Item should be timed out")
	}

	//test get within timeout
	cache.Add(TEST, TEST, 1*time.Millisecond)
	result, found := cache.Get(TEST)
	if !found {
		t.Error("([error] Segment SNMP-Cache Item not found")
	}
	resultString, ok := result.(string)
	if !ok {
		t.Error("([error] Segment SNMP-Cache changed item type")
	}
	if resultString != TEST {
		t.Error("([error] Segment SNMP-Cache changed item value")
	}

	time.Sleep(250 * time.Microsecond) //Item should still be in cache
	cache.Add(TEST, TEST, 1*time.Millisecond)
	result, found = cache.Get(TEST)
	if !found {
		t.Error("([error] Segment SNMP-Cache Item not found")
	}
	resultString, ok = result.(string)
	if !ok {
		t.Error("([error] Segment SNMP-Cache changed item type")
	}
	if resultString != TEST {
		t.Error("([error] Segment SNMP-Cache changed item value")
	}

	time.Sleep(1 * time.Millisecond) //item should no longer be in cache
	_, found = cache.Get(TEST)
	if found {
		t.Error("([error] Segment SNMP-Cache Item should be timed out")
	}

	if found {
		t.Error("([error] Segment SNMP-Cache Item should be timed out")
	}
}
