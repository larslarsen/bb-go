package attachment

import "testing"

func FuzzMEDIA001M2BRecord(f *testing.F) {
	f.Add([]byte(`{"version":1,"state":"ready","id":"01010101010101010101010101010101","cid":"bafkreibm6jg3ux5qum57eskivw3k5c4j6t7jvvwjjl6ue6zncuagorcz5y","byteLength":1}`))
	f.Add([]byte(`{"version":null,"state":"ready","id":"01010101010101010101010101010101","cid":"bafkreibm6jg3ux5qum57eskivw3k5c4j6t7jvvwjjl6ue6zncuagorcz5y","byteLength":1}`))
	f.Add([]byte(`{"version":1,"state":null,"id":"01010101010101010101010101010101","cid":"bafkreibm6jg3ux5qum57eskivw3k5c4j6t7jvvwjjl6ue6zncuagorcz5y","byteLength":1}`))
	f.Add([]byte(`{"version":1,"state":"ready","id":null,"cid":"bafkreibm6jg3ux5qum57eskivw3k5c4j6t7jvvwjjl6ue6zncuagorcz5y","byteLength":1}`))
	f.Add([]byte(`{"version":1,"state":"ready","id":"01010101010101010101010101010101","cid":null,"byteLength":1}`))
	f.Add([]byte(`{"version":1,"state":"ready","id":"01010101010101010101010101010101","cid":"bafkreibm6jg3ux5qum57eskivw3k5c4j6t7jvvwjjl6ue6zncuagorcz5y","byteLength":null}`))
	f.Add([]byte{})
	f.Add(make([]byte, maxRecordBytes+1))
	f.Fuzz(func(t *testing.T, data []byte) {
		record, err := decodeRecord(data)
		if err != nil {
			return
		}
		encoded, err := encodeRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := decodeRecord(encoded)
		if err != nil || roundTrip != record {
			t.Fatalf("round trip = %+v, %v", roundTrip, err)
		}
	})
}
