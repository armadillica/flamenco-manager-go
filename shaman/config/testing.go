package config

/* ***** BEGIN GPL LICENSE BLOCK *****
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software Foundation,
 * Inc., 59 Temple Place - Suite 330, Boston, MA  02111-1307, USA.
 *
 * ***** END GPL LICENCE BLOCK *****
 *
 * (c) 2019, Blender Foundation - Sybren A. Stüvel
 */

import (
	"io/ioutil"
	"os"
	"path"
	"time"
)

// CreateTestConfig creates a configuration + cleanup function.
func CreateTestConfig() (conf Config, cleanup func()) {
	tempDir, err := ioutil.TempDir("", "shaman-test-")
	if err != nil {
		panic(err)
	}

	conf = Config{
		TestTempDir:   tempDir,
		FileStorePath: path.Join(tempDir, "file-store"),
		CheckoutPath:  path.Join(tempDir, "checkout"),

		GarbageCollect: GarbageCollect{
			Period:            8 * time.Hour,
			MaxAge:            31 * 24 * time.Hour,
			ExtraCheckoutDirs: []string{},
		},
	}

	cleanup = func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(err)
		}
	}
	return
}
