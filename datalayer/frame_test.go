package datalayer

import (
	"reflect"
	"testing"
)

func TestFrame_Marshal(t *testing.T) {
	cases := []struct {
		data     frame
		expected []byte
	}{
		{
			// no error
			data: frame{
				start: startByte,
				dest:  0,
				src:   1,
				fType: iFrame,
				len:   6,
				data:  []byte("abcdef"),
				stop:  stopByte,
			},
			expected: []byte{startByte, 0, 1, iFrame, 6, 'a', 'b', 'c', 'd', 'e', 'f', stopByte},
		},
		{
			data: frame{
				start: startByte,
				dest:  0,
				src:   1,
				fType: iFrame,
				len:   16,
				data:  []byte(`{"nick":"asdf"}`),
				stop:  stopByte,
			},
			expected: []byte{startByte, 0, 1, iFrame, 16,
				'{', '"', 'n', 'i', 'c', 'k', '"', ':', '"', 'a', 's', 'd', 'f', '"', '}', stopByte},
		},
	}

	for i, c := range cases {
		if got := c.data.Marshal(); !reflect.DeepEqual(got, c.expected) {
			t.Errorf("[%d] binary data don't match: got '%s', expected '%s'", i, got, c.expected)
		}
	}
}

func TestFrame_Unmarshal(t *testing.T) {
	cases := []struct {
		data          []byte
		expectedFrame frame
		expectedErr   error
	}{
		{
			// no error
			data: []byte{startByte, 0, 1, iFrame, 6, 'a', 'b', 'c', 'd', 'e', 'f', stopByte},
			expectedFrame: frame{
				start: startByte,
				dest:  0,
				src:   1,
				fType: iFrame,
				len:   6,
				data:  []byte("abcdef"),
				stop:  stopByte,
			},
			expectedErr: nil,
		},
	}

	for i, c := range cases {
		var f frame
		if gotErr := f.Unmarshal(c.data); gotErr != c.expectedErr {
			t.Errorf("[%d] wrong error: got '%s', expected '%s'", i, gotErr, c.expectedErr)
		}
		if !reflect.DeepEqual(f, c.expectedFrame) {
			t.Errorf("[%d] frames don't match: got %+v, expected %+v", i, f, c.expectedFrame)
		}
	}
}
