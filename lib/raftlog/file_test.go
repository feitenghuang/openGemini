/*
Copyright 2024 Huawei Cloud Computing Technologies Co., Ltd.

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

package raftlog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openGemini/openGemini/lib/fileops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type badSeeker struct {
	fileops.File
}

func (badSeeker) Name() string {
	return "test file"
}

func (badSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("mock seek error")
}

type badWriter struct {
	badSeeker
}

func (badWriter) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (badWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock write error")
}

func TestFileWrap_OpenFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Run("OpenFile", func(t *testing.T) {
		fw, err := OpenFile(filepath.Join(tmpDir, "test"), os.O_CREATE|os.O_RDWR, 1000)
		require.EqualError(t, err, NewFile.Error())
		defer fw.Close()

		assert.Contains(t, fw.Name(), "test")
		assert.Equal(t, 1000, fw.Size())
	})

	t.Run("OpenFile_twice", func(t *testing.T) {
		fw, err := OpenFile(filepath.Join(tmpDir, "test2"), os.O_CREATE|os.O_RDWR, 1000)
		require.EqualError(t, err, NewFile.Error())
		defer fw.Close()

		assert.Contains(t, fw.Name(), "test")

		fw2, err := OpenFile(filepath.Join(tmpDir, "test2"), os.O_CREATE|os.O_RDWR, 1000)
		require.NoError(t, err)
		defer fw2.Close()

		assert.Contains(t, fw2.Name(), "test")
		assert.Equal(t, 1000, fw2.Size())

		fw3 := &FileWrap{}
		require.Equal(t, "", fw3.Name())
	})
}

func TestFileWrap_Write_WriteAt(t *testing.T) {
	tmpDir := t.TempDir()
	t.Run("WriteAt_Success", func(t *testing.T) {
		file, err := fileops.OpenFile(filepath.Join(tmpDir, "test"), os.O_CREATE|os.O_RDWR, 0640)
		require.NoError(t, err)
		file.Write([]byte("writeahead"))

		fw := &FileWrap{fd: file}
		defer fw.Close()
		fw.data = []byte("writeahead")

		n, err := fw.WriteAt(0, []byte("hello"))
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, []byte("hello"), fw.data[:5])
		var buff = make([]byte, 10)
		n, err = file.ReadAt(buff, 0)
		require.NoError(t, err)
		require.Equal(t, []byte("helloahead"), buff)
		require.Equal(t, []byte("helloahead"), fw.data)

		n, err = fw.Write([]byte("world"))
		require.NoError(t, err)
		require.Equal(t, 5, n)
		buff = make([]byte, 15)
		n, err = file.ReadAt(buff, 0)
		require.NoError(t, err)
		require.Equal(t, []byte("helloaheadworld"), buff)
		require.Equal(t, []byte("helloaheadworld"), fw.data)

		n, err = fw.WriteAt(5, []byte("faker"))
		require.NoError(t, err)
		require.Equal(t, 5, n)
		buff = make([]byte, 15)
		n, err = file.ReadAt(buff, 0)
		require.NoError(t, err)
		require.Equal(t, []byte("hellofakerworld"), buff)
		require.Equal(t, []byte("hellofakerworld"), fw.data)
	})

	t.Run("WriteAt_SeekError", func(t *testing.T) {
		fw := &FileWrap{fd: &badSeeker{}, data: make([]byte, 10)}

		_, err := fw.WriteAt(0, []byte("hello"))
		require.EqualError(t, err, "seek unreachable file:test file: mock seek error")
	})

	t.Run("Write_badWriter", func(t *testing.T) {
		fw := &FileWrap{fd: &badWriter{}, data: make([]byte, 10)}
		_, err := fw.Write([]byte("hello"))
		require.EqualError(t, err, "write failed:test file: mock write error")
	})

	t.Run("WriteAt_badWriter", func(t *testing.T) {
		fw := &FileWrap{fd: &badWriter{}, data: make([]byte, 10)}
		_, err := fw.WriteAt(0, []byte("hello"))
		require.EqualError(t, err, "write failed for file:test file: mock write error")
	})
}

func TestFileWrap_TrySync(t *testing.T) {
	t.Run("TrySync_fd_is_nil", func(t *testing.T) {
		fw := &FileWrap{}

		require.NoError(t, fw.TrySync())
	})
}
