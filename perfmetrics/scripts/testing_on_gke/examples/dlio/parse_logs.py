#!/usr/bin/env python

# Copyright 2018 The Kubernetes Authors.
# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# standard library imports
import argparse
import json
import os
import pprint
import subprocess
import sys

# local library imports
sys.path.append("../")
import dlio_workload
from utils.utils import get_memory, get_cpu, standard_timestamp, is_mash_installed, download_gcs_objects

_LOCAL_LOGS_LOCATION = "../../bin/dlio-logs/logs"

record = {
    "pod_name": "",
    "epoch": 0,
    "scenario": "",
    "train_au_percentage": 0,
    "duration": 0,
    "train_throughput_samples_per_second": 0,
    "train_throughput_mb_per_second": 0,
    "throughput_over_local_ssd": 0,
    "start": "",
    "end": "",
    "highest_memory": 0,
    "lowest_memory": 0,
    "highest_cpu": 0.0,
    "lowest_cpu": 0.0,
    "gcsfuse_mount_options": "",
}


def ensureDir(dirpath):
  try:
    os.makedirs(dirpath)
  except FileExistsError:
    pass


def downloadDlioOutputs(dlioWorkloads: set, instanceId: str) -> int:
  """Downloads instanceId-specific fio outputs for each fioWorkload locally.

  Outputs in the bucket are in the following sample object naming format
  (details in ./loading-test/templates/fio-tester.yaml).
    gs://<bucket>/logs/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/<scenario>/per_epoch_stats.json
    gs://<bucket>/logs/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/<scenario>/summary.json
    gs://<bucket>/logs/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/gcsfuse-generic/gcsfuse_mount_options

  These are downloaded locally as:
    <_LOCAL_LOGS_LOCATION>/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/<scenario>/per_epoch_stats.json
    <_LOCAL_LOGS_LOCATION>/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/<scenario>/summary.json
    <_LOCAL_LOGS_LOCATION>/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/gcsfuse-generic/gcsfuse_mount_options
  """
  for dlioWorkload in dlioWorkloads:
    srcObjects = f"gs://{dlioWorkload.bucket}/logs/{instanceId}"
    print(f"Downloading DLIO logs from the {srcObjects} ...")
    returncode, errorStr = download_gcs_objects(
        srcObjects, _LOCAL_LOGS_LOCATION
    )
    if returncode < 0:
      print(f"Failed to download DLIO logs from {srcObjects}: {errorStr}")
      return returncode
  return 0


def parseLogParserArguments() -> object:
  parser = argparse.ArgumentParser(
      prog="DLIO Unet3d test output parser",
      description=(
          "This program takes in a json workload configuration file and parses"
          " it for valid DLIO workloads and the locations of their test outputs"
          " on GCS. It downloads each such output object locally to"
          " {_LOCAL_LOGS_LOCATION} and parses them for DLIO test runs, and then"
          " dumps their output metrics into a CSV report file."
      ),
  )
  parser.add_argument(
      "--workload-config",
      help=(
          "A json configuration file to define workloads that were run to"
          " generate the outputs that should be parsed."
      ),
      required=True,
  )
  parser.add_argument(
      "--project-number",
      help=(
          "project-number (e.g. 93817472919) is needed to fetch the cpu/memory"
          " utilization data from GCP."
      ),
      required=True,
  )
  parser.add_argument(
      "--instance-id",
      help="unique string ID for current test-run",
      required=True,
  )
  parser.add_argument(
      "-o",
      "--output-file",
      metavar="Output file (CSV) path",
      help="File path of the output metrics (in CSV format)",
      default="output.csv",
  )
  return parser.parse_args()


if __name__ == "__main__":
  parseLogParserArguments()
  ensureDir(_LOCAL_LOGS_LOCATION)

  dlioWorkloads = dlio_workload.ParseTestConfigForDlioWorkloads(
      args.workload_config
  )
  downloadDlioOutputs(dlioWorkloads, args.instance_id)

  mash_installed = is_mash_installed()
  if not mash_installed:
    print("Mash is not installed, will skip parsing CPU and memory usage.")

  def createOutputScenariosFromDownloadedFiles():
    """Creates output records from the downloaded local files.

    The following creates a dict called 'output'
    from the downloaded fio output files, which are in the following format.

    <_LOCAL_LOGS_LOCATION>/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/<scenario>/per_epoch_stats.json
    <_LOCAL_LOGS_LOCATION>/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/<scenario>/summary.json
    <_LOCAL_LOGS_LOCATION>/<instanceId>/<numFilesTrain>-<recordLength>-<batchSize>-<hash>/gcsfuse-generic/gcsfuse_mount_options

    Output dict structure:
      "{num_files_train}-{mean_file_size}-{batch_size}":
          "mean_file_size": str
          "num_files_train": str
          "batch_size": str
          "records":
              "local-ssd": [record1, record2, record3, record4]
              "gcsfuse-generic": [record1, record2, record3, record4]
              "gcsfuse-file-cache": [record1, record2, record3, record4]
              "gcsfuse-no-file-cache": [record1, record2, record3, record4]
    """
    output = {}
    for root, _, files in os.walk(
        _LOCAL_LOGS_LOCATION + "/" + args.instance_id
    ):
      print(f"Parsing directory {root} ...")
      if files:
        # If directory contains gcsfuse_mount_options file, then parse gcsfuse
        # mount options from it in record.
        gcsfuse_mount_options = ""
        gcsfuse_mount_options_file = root + "/gcsfuse_mount_options"
        if os.path.isfile(gcsfuse_mount_options_file):
          with open(gcsfuse_mount_options_file) as f:
            gcsfuse_mount_options = f.read().strip()

        per_epoch_stats_file = root + "/per_epoch_stats.json"
        summary_file = root + "/summary.json"

        # Load per_epoch_stats.json .
        with open(per_epoch_stats_file, "r") as f:
          try:
            per_epoch_stats_data = json.load(f)
          except:
            print(f"failed to json-parse {per_epoch_stats_file}")
            continue

        # Load summary.json .
        with open(summary_file, "r") as f:
          try:
            summary_data = json.load(f)
          except:
            print(f"failed to json-parse {summary_file}")
            continue

        for i in range(summary_data["epochs"]):
          # Get numFilesTrain, recordLength, batchSize from the file/dir path.
          key = root.split("/")[-2]
          key_split = key.split("-")

          if key not in output:
            output[key] = {
                "num_files_train": key_split[-4],
                "mean_file_size": key_split[-3],
                "batch_size": key_split[-2],
                "records": {
                    "local-ssd": [],
                    "gcsfuse-generic": [],
                    "gcsfuse-file-cache": [],
                    "gcsfuse-no-file-cache": [],
                },
            }

          # Create a record for this key.
          r = record.copy()
          r["pod_name"] = summary_data["hostname"]
          r["epoch"] = i + 1
          r["scenario"] = root.split("/")[-1]
          r["train_au_percentage"] = round(
              summary_data["metric"]["train_au_percentage"][i], 2
          )
          r["duration"] = int(
              float(per_epoch_stats_data[str(i + 1)]["duration"])
          )
          r["train_throughput_samples_per_second"] = int(
              summary_data["metric"]["train_throughput_samples_per_second"][i]
          )
          r["train_throughput_mb_per_second"] = int(
              r["train_throughput_samples_per_second"]
              * int(output[key]["mean_file_size"])
              / (1024**2)
          )
          r["start"] = standard_timestamp(
              per_epoch_stats_data[str(i + 1)]["start"]
          )
          r["end"] = standard_timestamp(per_epoch_stats_data[str(i + 1)]["end"])
          if r["scenario"] != "local-ssd" and mash_installed:
            r["lowest_memory"], r["highest_memory"] = get_memory(
                r["pod_name"],
                r["start"],
                r["end"],
                project_number=args.project_number,
            )
            r["lowest_cpu"], r["highest_cpu"] = get_cpu(
                r["pod_name"],
                r["start"],
                r["end"],
                project_number=args.project_number,
            )
            pass

          r["gcsfuse_mount_options"] = gcsfuse_mount_options

          # This print is for debugging in case something goes wrong.
          pprint.pprint(r)

          # If a slot for record for this particular epoch has not been created yet,
          # append enough empty records to make a slot.
          while len(output[key]["records"][r["scenario"]]) < i + 1:
            output[key]["records"][r["scenario"]].append({})

          # Insert the record at the appropriate slot.
          output[key]["records"][r["scenario"]][i] = r

  createOutputScenariosFromDownloadedFiles()

  scenario_order = [
      "local-ssd",
      "gcsfuse-generic",
      "gcsfuse-no-file-cache",
      "gcsfuse-file-cache",
  ]

  def writeRecordsToCsvOutputFile(output_file_path: str):
    output_file = open(output_file_path, "a")
    output_file.write(
        "File Size,File #,Total Size (GB),Batch Size,Scenario,Epoch,Duration"
        " (s),GPU Utilization (%),Throughput (sample/s),Throughput"
        " (MB/s),Throughput over Local SSD (%),GCSFuse Lowest Memory"
        " (MB),GCSFuse Highest Memory (MB),GCSFuse Lowest CPU (core),GCSFuse"
        " Highest CPU (core),Pod,Start,End,GcsfuseMountOptions,InstanceID\n"
    )

    for key in output:
      record_set = output[key]
      total_size = int(
          int(record_set["mean_file_size"])
          * int(record_set["num_files_train"])
          / (1024**3)
      )

      for scenario in scenario_order:
        if scenario not in record_set["records"]:
          print(f"{scenario} not in output so skipping")
          continue
        if "local-ssd" in record_set["records"] and (
            len(record_set["records"]["local-ssd"])
            == len(record_set["records"][scenario])
        ):
          for i in range(len(record_set["records"]["local-ssd"])):
            r = record_set["records"][scenario][i]
            try:
              r["throughput_over_local_ssd"] = round(
                  r["train_throughput_mb_per_second"]
                  / record_set["records"]["local-ssd"][i][
                      "train_throughput_mb_per_second"
                  ]
                  * 100,
                  2,
              )
            except ZeroDivisionError:
              print("Got ZeroDivisionError. Ignoring it.")
              r["throughput_over_local_ssd"] = 0
            except:
              raise
            output_file.write(
                f"{record_set['mean_file_size']},{record_set['num_files_train']},{total_size},{record_set['batch_size']},{scenario},"
            )
            output_file.write(
                f"{r['epoch']},{r['duration']},{r['train_au_percentage']},{r['train_throughput_samples_per_second']},{r['train_throughput_mb_per_second']},{r['throughput_over_local_ssd']},{r['lowest_memory']},{r['highest_memory']},{r['lowest_cpu']},{r['highest_cpu']},{r['pod_name']},{r['start']},{r['end']},\"{r['gcsfuse_mount_options']}\",{args.instance_id}\n"
            )
        else:
          for i in range(len(record_set["records"][scenario])):
            r = record_set["records"][scenario][i]
            r["throughput_over_local_ssd"] = "NA"
            output_file.write(
                f"{record_set['mean_file_size']},{record_set['num_files_train']},{total_size},{record_set['batch_size']},{scenario},"
            )
            output_file.write(
                f"{r['epoch']},{r['duration']},{r['train_au_percentage']},{r['train_throughput_samples_per_second']},{r['train_throughput_mb_per_second']},{r['throughput_over_local_ssd']},{r['lowest_memory']},{r['highest_memory']},{r['lowest_cpu']},{r['highest_cpu']},{r['pod_name']},{r['start']},{r['end']},\"{r['gcsfuse_mount_options']}\",{args.instance_id}\n"
            )

    output_file.close()

  output_file_path = args.output_file
  # Create the parent directory of output_file_path if doesn't
  # exist already.
  ensureDir(os.path.dirname(output_file_path))
  writeRecordsToCsvOutputFile(output_file_path)
