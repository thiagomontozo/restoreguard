# RPO and RTO semantics

Measured RPO is the age of the selected recovery point at drill/incident time: `drill time - snapshot completion time`. A four-hour-old snapshot measures four hours of RPO even if backups are nominally scheduled hourly. Future or missing timestamps are `INCONCLUSIVE`.

Measured RTO starts when the drill begins and ends only when the environment is ready, restore has completed, and all required validations have completed. Restore duration alone is not RTO. Tests use fixed timestamps/fake clocks for policy failure cases instead of long sleeps.

Each objective independently returns `PASS`, `FAIL`, or `INCONCLUSIVE`. Missing a target does not automatically mean a technical restore failure; the report shows restore, validations, RPO, and RTO separately.
