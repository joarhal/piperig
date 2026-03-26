#!/bin/bash
trap 'echo "SIGTERM received"; exit 0' TERM
sleep 30
