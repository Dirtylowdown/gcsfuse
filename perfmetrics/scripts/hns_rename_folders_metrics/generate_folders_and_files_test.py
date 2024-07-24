# Copyright 2024 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
import subprocess
import unittest
import generate_folders_and_files
from mock import patch, call


class TestCheckForConfigFileInconsistency(unittest.TestCase):
  def test_missing_bucket_name(self):
    config = {}
    result = generate_folders_and_files.check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_missing_keys_from_folder(self):
    config = {
        "name": "test_bucket",
        "folders": {}
    }
    result = generate_folders_and_files.check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_missing_keys_from_nested_folder(self):
    config = {
        "name": "test_bucket",
        "nested_folders": {}
    }
    result = generate_folders_and_files.check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_folders_num_folder_mismatch(self):
    config = {
        "name": "test_bucket",
        "folders": {
            "num_folders": 2,
            "folder_structure": [
                {
                    "name": "test_folder",
                    "num_files": 10,
                    "file_name_prefix": "file",
                    "file_size": "1kb"
                }
            ]
        }
    }
    result = generate_folders_and_files.check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_nested_folders_num_folder_mismatch(self):
    config = {
        "name": "test_bucket",
        "nested_folders": {
            "folder_name": "test_nested_folder",
            "num_folders": 2,
            "folder_structure": [
                {
                    "name": "test_folder",
                    "num_files": 10,
                    "file_name_prefix": "file",
                    "file_size": "1kb"
                }
            ]
        }
    }
    result = generate_folders_and_files.check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_valid_config(self):
    config = {
        "name": "test_bucket",
        "folders": {
            "num_folders": 1,
            "folder_structure": [
                {
                    "name": "test_folder",
                    "num_files": 1,
                    "file_name_prefix": "file",
                    "file_size": "1kb"
                }
            ]
        },
        "nested_folders": {
            "folder_name": "nested",
            "num_folders": 1,
            "folder_structure": [
                {
                    "name": "test_folder",
                    "num_files": 1,
                    "file_name_prefix": "file",
                    "file_size": "1kb"
                }
            ]
        }
    }
    result = generate_folders_and_files.check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 0)


class TestListDirectory(unittest.TestCase):

  @patch('subprocess.check_output')
  @patch('subprocess.call')
  @patch('generate_folders_and_files.logmessage')
  def test_listing_at_non_existent_path(self, mock_logmessage,
      mock_subprocess_call, mock_check_output):
    mock_check_output.side_effect = subprocess.CalledProcessError(
        returncode=1,
        cmd="gcloud storage ls gs://fake_bkt",
        output=b'Error while listing')

    dir_list = generate_folders_and_files.list_directory("gs://fake_bkt")

    self.assertEqual(dir_list, None)
    mock_logmessage.assert_called_once_with('Error while listing')
    mock_subprocess_call.assert_called_once_with('bash', shell=True)

  @patch('subprocess.check_output')
  def test_listing_directory(self, mock_check_output):
    mock_check_output.return_value = b'gs://fake_bkt/fake_folder_0/\n' \
                                     b'gs://fake_bkt/fake_folder_1/\n' \
                                     b'gs://fake_bkt/nested_fake_folder/\n'
    expected_dir_list = ["gs://fake_bkt/fake_folder_0/",
                         "gs://fake_bkt/fake_folder_1/",
                         "gs://fake_bkt/nested_fake_folder/"]

    dir_list = generate_folders_and_files.list_directory("gs://fake_bkt")

    self.assertEqual(dir_list, expected_dir_list)


if __name__ == '__main__':
  unittest.main()
