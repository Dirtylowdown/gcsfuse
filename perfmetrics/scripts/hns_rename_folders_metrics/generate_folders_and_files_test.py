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
    result = generate_folders_and_files._check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_missing_keys_from_folder(self):
    config = {
        "name": "test_bucket",
        "folders": {}
    }
    result = generate_folders_and_files._check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 1)

  def test_missing_keys_from_nested_folder(self):
    config = {
        "name": "test_bucket",
        "nested_folders": {}
    }
    result = generate_folders_and_files._check_for_config_file_inconsistency(
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
    result = generate_folders_and_files._check_for_config_file_inconsistency(
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
    result = generate_folders_and_files._check_for_config_file_inconsistency(
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
    result = generate_folders_and_files._check_for_config_file_inconsistency(
        config)
    self.assertEqual(result, 0)


class TestListDirectory(unittest.TestCase):

  @patch('subprocess.check_output')
  @patch('generate_folders_and_files._logmessage')
  def test_listing_at_non_existent_path(self, mock_logmessage,mock_check_output):
    mock_check_output.side_effect = subprocess.CalledProcessError(
        returncode=1,
        cmd="gcloud storage ls gs://fake_bkt",
        output=b'Error while listing')

    dir_list = generate_folders_and_files._list_directory("gs://fake_bkt")

    self.assertEqual(dir_list, None)
    mock_logmessage.assert_called_once_with('Error while listing')

  @patch('subprocess.check_output')
  def test_listing_directory(self, mock_check_output):
    mock_check_output.return_value = b'gs://fake_bkt/fake_folder_0/\n' \
                                     b'gs://fake_bkt/fake_folder_1/\n' \
                                     b'gs://fake_bkt/nested_fake_folder/\n'
    expected_dir_list = ["gs://fake_bkt/fake_folder_0/",
                         "gs://fake_bkt/fake_folder_1/",
                         "gs://fake_bkt/nested_fake_folder/"]

    dir_list = generate_folders_and_files._list_directory("gs://fake_bkt")

    self.assertEqual(dir_list, expected_dir_list)


class TestCompareFolderStructure(unittest.TestCase):

  @patch('generate_folders_and_files._list_directory')
  def test_folder_structure_matches(self,mock_listdir):
    mock_listdir.return_value=['test_file_1.txt']
    test_folder={
        "name": "test_folder",
        "num_files": 1,
        "file_name_prefix": "test_file",
        "file_size": "1kb"
    }
    test_folder_url='gs://temp_folder_url'

    match = generate_folders_and_files._compare_folder_structure(test_folder, test_folder_url)

    self.assertEqual(match,True)

  @patch('generate_folders_and_files._list_directory')
  def test_folder_structure_mismatches(self,mock_listdir):
    mock_listdir.return_value=['test_file_1.txt']
    test_folder={
        "name": "test_folder",
        "num_files": 2,
        "file_name_prefix": "test_file",
        "file_size": "1kb"
    }
    test_folder_url='gs://temp_folder_url'

    match = generate_folders_and_files._compare_folder_structure(test_folder, test_folder_url)

    self.assertEqual(match,False)

  @patch('generate_folders_and_files._list_directory')
  def test_folder_does_not_exist_in_gcs_bucket(self,mock_listdir):
    mock_listdir.side_effect=subprocess.CalledProcessError(
        returncode=1,
        cmd="gcloud storage ls gs://fake_bkt/folder_does_not_exist",
        output=b'Error while listing')
    test_folder={
        "name": "test_folder",
        "num_files": 1,
        "file_name_prefix": "test_file",
        "file_size": "1kb"
    }
    test_folder_url='gs://fake_bkt/folder_does_not_exist'

    match = generate_folders_and_files._compare_folder_structure(test_folder, test_folder_url)

    self.assertEqual(match,False)
    self.assertRaises(subprocess.CalledProcessError)


class TestCheckIfDirStructureExists(unittest.TestCase):

  @patch("generate_folders_and_files._list_directory")
  def test_dir_already_exists_in_gcs_bucket(self, mock_list_directory):
    mock_list_directory.side_effect = [
        ["gs://test_bucket/test_folder/", "gs://test_bucket/nested/"],
        ["gs://test_bucket/test_folder/file_1.txt"],
        ["gs://test_bucket/nested/test_folder/"],
        ["gs://test_bucket/nested/test_folder/file_1.txt"]
    ]
    dir_config = {
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

    dir_present = generate_folders_and_files._check_if_dir_structure_exists(
      dir_config)

    self.assertEqual(dir_present, 1)

  @patch("generate_folders_and_files._list_directory")
  def test_dir_does_not_exist_in_gcs_bucket(self, mock_list_directory):
    mock_list_directory.side_effect = [
        ["gs://test_bucket/test_folder/", "gs://test_bucket/nested/"],
        ["gs://test_bucket/test_folder/file_1.txt",
         "gs://test_bucket/test_folder/file_1.txt"],
        ["gs://test_bucket/nested/test_folder/"],
        ["gs://test_bucket/nested/test_folder/file_1.txt"]
    ]
    dir_config = {
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

    dir_present = generate_folders_and_files._check_if_dir_structure_exists(
      dir_config)

    self.assertEqual(dir_present, 0)


class TestDeleteExistingDataInGcsBucket(unittest.TestCase):

  @patch('subprocess.check_output')
  @patch('generate_folders_and_files._logmessage')
  def test_deleting_failure(self, mock_logmessage,
      mock_check_output):
    mock_check_output.side_effect = subprocess.CalledProcessError(
        returncode=1,
        cmd="gcloud alpha storage rm -r gs://fake_bkt",
        output=b'Error while deleting')

    exit_code = generate_folders_and_files\
      ._delete_existing_data_in_gcs_bucket("fake_bkt")

    self.assertEqual(exit_code, 1)
    mock_logmessage.assert_called_once_with('Error while deleting')

  @patch('subprocess.check_output')
  def test_deleting_success(self,mock_check_output):
    mock_check_output.return_value = 0

    exit_code = generate_folders_and_files \
      ._delete_existing_data_in_gcs_bucket("fake_bkt")

    self.assertEqual(exit_code, 0)


if __name__ == '__main__':
  unittest.main()
