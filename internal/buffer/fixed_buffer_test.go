//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 THL A29 Limited, a Tencent company.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

package buffer

import (
	"bytes"
	"io"
	"testing"
)

func TestFixedReadBuffer(t *testing.T) {
	// Table-driven test cases
	tests := []struct {
		name        string                             // Test name
		initData    []byte                             // Data to initialize buffer with
		readSize    int                                // Size to read
		expected    []byte                             // Expected read result
		expectError error                              // Expected error
		validate    func(*testing.T, *FixedReadBuffer) // Additional validation
	}{
		{
			name:        "Read data smaller than buffer",
			initData:    []byte("hello"),
			readSize:    5,
			expected:    []byte("hello"),
			expectError: nil,
		},
		{
			name:        "Read request larger than available data",
			initData:    []byte("hello"),
			readSize:    10,
			expected:    []byte("hello"),
			expectError: nil,
		},
		{
			name:        "Multiple reads from buffer",
			initData:    []byte("helloworld"),
			readSize:    5,
			expected:    []byte("hello"),
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Second read
				data := make([]byte, 5)
				n, err := buf.Read(data)
				if err != nil || n != 5 || !bytes.Equal(data[:n], []byte("world")) {
					t.Errorf("Second read failed: err=%v, n=%d, data=%v", err, n, data[:n])
				}
			},
		},
		{
			name:        "Partial data read",
			initData:    []byte("hello"),
			readSize:    3,
			expected:    []byte("hel"),
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Check remaining data
				remain := buf.LenRead()
				if remain != 2 {
					t.Errorf("Remaining data should be 2, got %d", remain)
				}
			},
		},
		{
			name:        "Read from empty buffer",
			initData:    []byte{},
			readSize:    1,
			expected:    []byte{},
			expectError: io.EOF,
		},
		{
			name:        "Read after all data consumed",
			initData:    []byte("hello"),
			readSize:    5,
			expected:    []byte("hello"),
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Try to read again after all data consumed
				data := make([]byte, 1)
				n, err := buf.Read(data)
				if n != 0 || err != io.EOF {
					t.Errorf("Expected EOF after all data consumed, got: err=%v, n=%d", err, n)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create and initialize buffer
			buf := &FixedReadBuffer{}
			buf.Initialize(tt.initData)

			// Read data
			data := make([]byte, tt.readSize)
			n, err := buf.Read(data)

			// Verify read results
			if err != tt.expectError && !(err == nil && tt.expectError == nil) {
				t.Errorf("Expected error %v, got %v", tt.expectError, err)
			}

			// Adjust actual read data length
			data = data[:n]
			if !bytes.Equal(data, tt.expected) {
				t.Errorf("Expected data %v, got %v", tt.expected, data)
			}

			// Execute additional validation if provided
			if tt.validate != nil {
				tt.validate(t, buf)
			}
		})
	}
}

func TestFixedReadBuffer_Peek(t *testing.T) {
	tests := []struct {
		name        string
		initData    []byte
		peekSize    int
		expected    []byte
		expectError error
		validate    func(*testing.T, *FixedReadBuffer)
	}{
		{
			name:        "Peek available data",
			initData:    []byte("hello"),
			peekSize:    3,
			expected:    []byte("hel"),
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Ensure peek didn't advance the buffer
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after peek, got %d", buf.LenRead())
				}
			},
		},
		{
			name:        "Peek more data than available",
			initData:    []byte("hello"),
			peekSize:    10,
			expected:    nil,
			expectError: io.EOF,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Ensure peek didn't change buffer state even on error
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after failed peek, got %d", buf.LenRead())
				}
			},
		},
		{
			name:        "Peek with negative size",
			initData:    []byte("hello"),
			peekSize:    -1,
			expected:    nil,
			expectError: ErrInvalidParam,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Ensure peek didn't change buffer state on invalid parameter
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after invalid peek, got %d", buf.LenRead())
				}
			},
		},
		{
			name:        "Peek with zero size",
			initData:    []byte("hello"),
			peekSize:    0,
			expected:    []byte{},
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Ensure peek didn't change buffer state on zero-size peek
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after zero-size peek, got %d", buf.LenRead())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &FixedReadBuffer{}
			buf.Initialize(tt.initData)

			// Check initial length
			initialLen := buf.LenRead()

			data, err := buf.Peek(tt.peekSize)

			// Verify length hasn't changed
			if buf.LenRead() != initialLen {
				t.Errorf("Peek changed buffer length: before=%d, after=%d", initialLen, buf.LenRead())
			}

			if err != tt.expectError {
				t.Errorf("Expected error %v, got %v", tt.expectError, err)
			}

			if !bytes.Equal(data, tt.expected) {
				t.Errorf("Expected data %v, got %v", tt.expected, data)
			}

			if tt.validate != nil {
				tt.validate(t, buf)
			}
		})
	}
}

func TestFixedReadBuffer_Skip(t *testing.T) {
	tests := []struct {
		name        string
		initData    []byte
		skipSize    int
		expectError error
		validate    func(*testing.T, *FixedReadBuffer)
	}{
		{
			name:        "Skip available data",
			initData:    []byte("hello"),
			skipSize:    3,
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Check buffer state after skip
				if buf.LenRead() != 2 {
					t.Errorf("Expected length 2 after skip, got %d", buf.LenRead())
				}

				// Read remaining data
				data := make([]byte, 2)
				n, err := buf.Read(data)
				if err != nil || n != 2 || !bytes.Equal(data[:n], []byte("lo")) {
					t.Errorf("Failed to read remaining data: err=%v, n=%d, data=%v", err, n, data[:n])
				}
			},
		},
		{
			name:        "Skip more data than available",
			initData:    []byte("hello"),
			skipSize:    10,
			expectError: io.EOF,
		},
		{
			name:        "Skip with negative size",
			initData:    []byte("hello"),
			skipSize:    -1,
			expectError: ErrInvalidParam,
		},
		{
			name:        "Skip with zero size",
			initData:    []byte("hello"),
			skipSize:    0,
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Buffer should be unchanged
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after skip(0), got %d", buf.LenRead())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &FixedReadBuffer{}
			buf.Initialize(tt.initData)

			err := buf.Skip(tt.skipSize)

			if err != tt.expectError {
				t.Errorf("Expected error %v, got %v", tt.expectError, err)
			}

			if tt.validate != nil {
				tt.validate(t, buf)
			}
		})
	}
}

func TestFixedReadBuffer_Next(t *testing.T) {
	tests := []struct {
		name        string
		initData    []byte
		nextSize    int
		expected    []byte
		expectError error
		validate    func(*testing.T, *FixedReadBuffer)
	}{
		{
			name:        "Next available data",
			initData:    []byte("hello"),
			nextSize:    3,
			expected:    []byte("hel"),
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Check buffer state after next
				if buf.LenRead() != 2 {
					t.Errorf("Expected length 2 after next, got %d", buf.LenRead())
				}

				// Read remaining data with Next
				data, err := buf.Next(2)
				if err != nil || !bytes.Equal(data, []byte("lo")) {
					t.Errorf("Failed to read remaining data: err=%v, data=%v", err, data)
				}
			},
		},
		{
			name:        "Next more data than available",
			initData:    []byte("hello"),
			nextSize:    10,
			expected:    nil,
			expectError: io.EOF,
		},
		{
			name:        "Next with negative size",
			initData:    []byte("hello"),
			nextSize:    -1,
			expected:    nil,
			expectError: ErrInvalidParam,
		},
		{
			name:        "Next with zero size",
			initData:    []byte("hello"),
			nextSize:    0,
			expected:    []byte{},
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Buffer should be unchanged
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after next(0), got %d", buf.LenRead())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &FixedReadBuffer{}
			buf.Initialize(tt.initData)

			data, err := buf.Next(tt.nextSize)

			if err != tt.expectError {
				t.Errorf("Expected error %v, got %v", tt.expectError, err)
			}

			if !bytes.Equal(data, tt.expected) {
				t.Errorf("Expected data %v, got %v", tt.expected, data)
			}

			if tt.validate != nil {
				tt.validate(t, buf)
			}
		})
	}
}

func TestFixedReadBuffer_ReadN(t *testing.T) {
	tests := []struct {
		name        string
		initData    []byte
		readSize    int
		expected    []byte
		expectError error
		validate    func(*testing.T, *FixedReadBuffer)
	}{
		{
			name:        "ReadN available data",
			initData:    []byte("hello"),
			readSize:    3,
			expected:    []byte("hel"),
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Check buffer state after ReadN
				if buf.LenRead() != 2 {
					t.Errorf("Expected length 2 after ReadN, got %d", buf.LenRead())
				}

				// Read remaining data
				data, err := buf.ReadN(2)
				if err != nil || !bytes.Equal(data, []byte("lo")) {
					t.Errorf("Failed to read remaining data: err=%v, data=%v", err, data)
				}
			},
		},
		{
			name:        "ReadN more data than available",
			initData:    []byte("hello"),
			readSize:    10,
			expected:    nil,
			expectError: io.EOF,
		},
		{
			name:        "ReadN with negative size",
			initData:    []byte("hello"),
			readSize:    -1,
			expected:    nil,
			expectError: ErrInvalidParam,
		},
		{
			name:        "ReadN with zero size",
			initData:    []byte("hello"),
			readSize:    0,
			expected:    []byte{},
			expectError: nil,
			validate: func(t *testing.T, buf *FixedReadBuffer) {
				// Buffer should be unchanged
				if buf.LenRead() != 5 {
					t.Errorf("Buffer length should be unchanged after ReadN(0), got %d", buf.LenRead())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &FixedReadBuffer{}
			buf.Initialize(tt.initData)

			data, err := buf.ReadN(tt.readSize)

			if err != tt.expectError {
				t.Errorf("Expected error %v, got %v", tt.expectError, err)
			}

			if !bytes.Equal(data, tt.expected) {
				t.Errorf("Expected data %v, got %v", tt.expected, data)
			}

			if tt.validate != nil {
				tt.validate(t, buf)
			}
		})
	}
}

func TestFixedReadBuffer_LenAndPos(t *testing.T) {
	buf := &FixedReadBuffer{}
	data := []byte("hello world")

	buf.Initialize(data)

	// Test initial state
	if buf.LenRead() != len(data) {
		t.Errorf("Expected length %d, got %d", len(data), buf.LenRead())
	}

	if buf.CurPos() != 0 {
		t.Errorf("Expected position 0, got %d", buf.CurPos())
	}

	// Read some data
	readSize := 5
	readBuf := make([]byte, readSize)
	n, err := buf.Read(readBuf)

	if err != nil || n != readSize {
		t.Errorf("Failed to read: err=%v, n=%d", err, n)
	}

	// Check updated state
	if buf.LenRead() != len(data)-readSize {
		t.Errorf("Expected length %d, got %d", len(data)-readSize, buf.LenRead())
	}

	if buf.CurPos() != readSize {
		t.Errorf("Expected position %d, got %d", readSize, buf.CurPos())
	}
}

// Test edge cases
func TestFixedReadBuffer_EdgeCases(t *testing.T) {
	t.Run("Zero size buffer", func(t *testing.T) {
		buf := &FixedReadBuffer{}
		buf.Initialize([]byte{})

		// Read should return EOF
		data := make([]byte, 1)
		n, err := buf.Read(data)
		if n != 0 || err != io.EOF {
			t.Errorf("Reading from empty buffer should return EOF")
		}
	})

	t.Run("Concurrent read safety", func(t *testing.T) {
		buf := &FixedReadBuffer{}
		buf.Initialize(make([]byte, 100))
		done := make(chan bool)

		// Start multiple concurrent read goroutines
		for i := 0; i < 5; i++ {
			go func() {
				data := make([]byte, 10)
				for j := 0; j < 10; j++ {
					_, err := buf.Read(data)
					if err != nil && err != io.EOF {
						t.Errorf("Failed to read data: err=%v", err)
					}
				}
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 5; i++ {
			<-done
		}
	})

	t.Run("Large data handling", func(t *testing.T) {
		buf := &FixedReadBuffer{}
		chunkSize := 1024
		chunkNum := 400
		largeData := make([]byte, chunkSize*chunkNum)
		for i := range largeData {
			largeData[i] = byte(i % chunkSize)
		}

		buf.Initialize(largeData)

		// Read in multiple chunks
		for i := 0; i < chunkNum; i++ {
			data := make([]byte, chunkSize)
			n, err := buf.Read(data)
			if err != nil || n != chunkSize {
				t.Errorf("Failed to read data: err=%v, n=%d", err, n)
			}

			// Verify data correctness
			for j := 0; j < chunkSize; j++ {
				expected := byte((i*chunkSize + j) % chunkSize)
				if data[j] != expected {
					t.Errorf("Data mismatch: index=%d, expected=%d, got=%d", j, expected, data[j])
					break
				}
			}
		}
	})
}
