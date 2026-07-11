import { useEffect, useState } from "react";
import { api } from "../lib/api";

type ApplianceInfo = {
  dataDir: string;
  masterKeyPath: string;
  databasePath: string;
  backupsPath: string;
};

type Props = {
  embedded?: boolean;
};

export default function DisasterRecovery({ embedded }: Props) {
  const [info, setInfo] = useState<ApplianceInfo | null>(null);

  useEffect(() => {
    api<ApplianceInfo>("/api/appliance")
      .then(setInfo)
      .catch(() => undefined);
  }, []);

  const dataDir = info?.dataDir || "/var/lib/boomerang";
  const masterKey = info?.masterKeyPath || `${dataDir}/secrets/master.key`;
  const dbPath = info?.databasePath || `${dataDir}/app.db`;
  const backupsPath = info?.backupsPath || `${dataDir}/backups`;

  const body = (
    <>
      <p className="muted settings-lead">
        Backups live on this appliance first, then optionally mirror to Cloudflare R2 (Settings →
        Off-site). Copy the data directory off-site so you can rebuild after hardware loss.
      </p>

      <div className="settings-blocks">
        <div className="settings-block callout warn">
          <h3>What to protect</h3>
          <p className="muted small">
            <strong>Never share</strong> <code>secrets/master.key</code> in email, chat, tickets, or
            git — it decrypts every backup and stored password on this appliance.
          </p>
          <dl className="path-list">
            <div>
              <dt>Master key</dt>
              <dd>
                <code>{masterKey}</code>
                <span className="muted small">
                  Decrypts backup files (<code>.enc</code>) and credentials in <code>app.db</code>
                </span>
              </dd>
            </div>
            <div>
              <dt>Database</dt>
              <dd>
                <code>{dbPath}</code>
                <span className="muted small">Targets, schedules, and config</span>
              </dd>
            </div>
            <div>
              <dt>Backup files</dt>
              <dd>
                <code>{backupsPath}/</code>
                <span className="muted small">All file and database versions</span>
              </dd>
            </div>
          </dl>
        </div>

        <div className="settings-block">
          <h3>Recommended practice</h3>
          <ul className="plain compact">
            <li>
              Enable <strong>Settings → Off-site</strong> to mirror to Cloudflare R2 automatically
              after each backup.
            </li>
            <li>
              Or sync <code>{dataDir}</code> to another host or NAS (rsync, rclone, snapshots).
            </li>
            <li>Keep a separate copy of <code>master.key</code> — without it, data is lost.</li>
            <li>Run a test restore on a spare VM once a year.</li>
          </ul>
        </div>

        <div className="settings-block">
          <h3>Restore on a new server</h3>
          <ol className="plain numbered compact">
            <li>
              On a <strong>new install</strong>, choose <strong>Restore from R2</strong> on the first-flight
              screen (before setting a password).
            </li>
            <li>
              Or install Boomerang, stop the service, copy the mirrored tree into{" "}
              <code>{dataDir}</code>, fix ownership, and start the service.
            </li>
            <li>
              Confirm <code>master.key</code> is in place (or set <code>BOOMERANG_MASTER_KEY</code>).
            </li>
            <li>
              <code>chown -R boomerang:boomerang {dataDir}</code>, then start the service.
            </li>
            <li>Update remote firewalls for the new appliance IP.</li>
            <li>Sign in with your previous admin password if you restored from R2 or a full data copy.</li>
          </ol>
        </div>
      </div>
    </>
  );

  if (embedded) return body;

  return (
    <section className="tile">
      <h2>Disaster recovery</h2>
      {body}
    </section>
  );
}
