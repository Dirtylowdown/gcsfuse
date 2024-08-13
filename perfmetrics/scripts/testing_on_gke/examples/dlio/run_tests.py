#!/usr/bin/env python

# Copyright 2018 The Kubernetes Authors.
# Copyright 2022 Google LLC
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

"""Generates and deploys helm charts for DLIO workloads.

This program takes in a json test-config file, finds out valid
DLIO workloads from it and generates and deploys a helm chart for
each valid DLIO workload.
"""

import argparse
import subprocess
import dlio_workload


def run_command(command: str):
  """Runs the given string command as a subprocess."""
  result = subprocess.run(command.split(' '), capture_output=True, text=True)
  print(result.stdout)
  print(result.stderr)


def escapeCommasInString(unescapedStr: str) -> str:
  """Returns equivalent string with ',' replaced with '\,' ."""
  return unescapedStr.replace(',', '\,')


def createHelmInstallCommands(
    dlioWorkloads: set,
    instanceId: str,
    machineType: str,
) -> list:
  """Creates helm install commands for the given dlioWorkload objects."""
  helm_commands = []
  for dlioWorkload in dlioWorkloads:
    for batchSize in dlioWorkload.batchSizes:
      chartName, podName, outputDirPrefix = dlio_workload.DlioChartNamePodName(
          dlioWorkload, instanceId, batchSize
      )
      commands = [
          f'helm install {chartName} unet3d-loading-test',
          f'--set bucketName={dlioWorkload.bucket}',
          f'--set scenario={dlioWorkload.scenario}',
          f'--set dlio.numFilesTrain={dlioWorkload.numFilesTrain}',
          f'--set dlio.recordLength={dlioWorkload.recordLength}',
          f'--set dlio.batchSize={batchSize}',
          f'--set instanceId={instanceId}',
          (
              '--set'
              f' gcsfuse.mountOptions={escapeCommasInString(dlioWorkload.gcsfuseMountOptions)}'
          ),
          f'--set nodeType={machineType}',
          f'--set podName={podName}',
          f'--set outputDirPrefix={outputDirPrefix}',
      ]

      helm_command = ' '.join(commands)
      helm_commands.append(helm_command)
  return helm_commands


def main(args) -> None:
  dlioWorkloads = dlio_workload.ParseTestConfigForDlioWorkloads(
      args.workload_config
  )
  helmInstallCommands = createHelmInstallCommands(
      dlioWorkloads,
      args.instance_id,
      args.machine_type,
  )
  for helmInstallCommand in helmInstallCommands:
    print(f'{helmInstallCommand}')
    if not args.dry_run:
      run_command(helmInstallCommand)


if __name__ == '__main__':
  parser = argparse.ArgumentParser(
      prog='DLIO Unet3d test runner',
      description=(
          'This program takes in a json test-config file, finds out valid DLIO'
          ' workloads from it and generates and deploys a helm chart for each'
          ' DLIO workload.'
      ),
  )
  parser.add_argument(
      '--workload-config',
      metavar='JSON workload configuration file path',
      help='Runs DLIO Unet3d tests from this JSON workload configuration file.',
      required=True,
  )
  parser.add_argument(
      '--instance-id',
      metavar='A unique string ID to represent the test-run',
      help=(
          'Set to a unique string ID for current test-run. Do not put spaces'
          ' in it.'
      ),
      required=True,
  )
  parser.add_argument(
      '--machine-type',
      metavar='Machine-type of the GCE VM or GKE cluster node',
      help='Machine-type of the GCE VM or GKE cluster node e.g. n2-standard-32',
      required=True,
  )
  parser.add_argument(
      '-n',
      '--dry-run',
      action='store_true',
      help=(
          'Only print out the test configurations that will run,'
          ' not actually run them.'
      ),
  )

  args = parser.parse_args()
  for argument in ['instance_id', 'machine_type']:
    value = getattr(args, argument)
    if len(value) == 0 or str.isspace(value):
      raise Exception(
          f'Argument {argument} (value="{value}") is empty or contains only'
          ' spaces.'
      )
    if ' ' in value:
      raise Exception(
          f'Argument {argument} (value="{value}") contains space in it, which'
          ' is not supported.'
      )

  main(args)
