#!/usr/bin/env bash
{
  echo "INFO: starting job"
  sleep 1
  echo "ERROR: connection refused"
  sleep 1
  echo "INFO: retrying connection"
  sleep 1
  echo "WARNING: slow response 5s"
  sleep 1
  echo "INFO: connected"
  sleep 1
  echo "error: query timeout"
  sleep 1
  echo "INFO: done"
} | nix run --refresh github:rhousand/output-monitor/feat/enhanced-filtering -- -t -C 1 ERROR WARNING INFO
