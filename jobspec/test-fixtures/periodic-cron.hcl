# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

job "foo" {
  periodic {
    cron             = "*/5 * * *"
    prohibit_overlap = true
    time_zone        = "Europe/Minsk"
  }
}
