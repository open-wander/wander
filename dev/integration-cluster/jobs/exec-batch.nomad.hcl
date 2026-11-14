# -------------------------------------------------------------------------------
# exec-batch -- exec driver isolation on whichever client takes it
#
# Project: Wander
#
# A batch task under the exec driver's chroot and cgroups v2 isolation. It
# prints exec-ok and its own cgroup, which the suite checks is under nomad.slice.
# -------------------------------------------------------------------------------

job "exec-batch" {
  datacenters = ["dc1"]
  type        = "batch"

  # ---------------------------------------------------------------------------
  # Task Group: g
  # ---------------------------------------------------------------------------

  group "g" {
    task "t" {
      driver = "exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "echo exec-ok; cat /proc/self/cgroup"]
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}
