# Failure recovery and reboot acceptance

Run only from the already elevated acceptance PowerShell.

1. Confirm the service is Running and the readiness endpoint responds.
2. Record its PID and run the orchestrator FailureRecovery stage with
   ConfirmMachineChanges.
3. Wait at least 40 seconds. Verify the first authored SCM recovery restart
   occurred, readiness returned, and no rapid restart loop exists.
4. Open the installed Server Manager. Confirm its status is truthful and Restart
   returns the service to Running if needed.
5. When the harness reaches the reboot gate, let the user approve the reboot.
6. After Windows starts, run Test-PostReboot.ps1. It waits up to 120 seconds and
   records service state, readiness, and startup timing without changing the
   machine.
