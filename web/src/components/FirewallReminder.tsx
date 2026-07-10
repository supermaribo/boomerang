import { useEffect, useState } from "react";
import { api } from "../App";

type ApplianceInfo = {
  sourceIPs: string[];
};

type Props = {
  targetHost?: string;
  port: number;
  protocol: string;
};

export default function FirewallReminder({ targetHost, port, protocol }: Props) {
  const [ips, setIps] = useState<string[]>([]);

  useEffect(() => {
    api<ApplianceInfo>("/api/appliance")
      .then((info) => setIps(info.sourceIPs || []))
      .catch(() => undefined);
  }, []);

  const hostLabel = targetHost?.trim() || "your server";

  return (
    <aside className="callout security" role="note">
      <h3>Firewall on the remote server</h3>
      <p>
        Boomerang connects <strong>outbound</strong> from this appliance to{" "}
        <code>
          {hostLabel}:{port}
        </code>{" "}
        ({protocol}). On the <strong>remote</strong> host, allow that traffic from this backup
        server&apos;s IP — not from the whole internet.
      </p>
      {ips.length > 0 ? (
        <p>
          This Boomerang&apos;s address(es):{" "}
          {ips.map((ip, i) => (
            <span key={ip}>
              {i > 0 && ", "}
              <code>{ip}</code>
            </span>
          ))}
        </p>
      ) : (
        <p className="muted small">
          Could not detect this server&apos;s IP automatically. Use the address your remote
          firewall sees when Boomerang connects (check your router/NAT if needed).
        </p>
      )}
      <ul className="plain small">
        <li>
          <strong>Do not</strong> open {protocol} to <code>0.0.0.0/0</code> or <code>::/0</code> —
          that exposes your site to the world.
        </li>
        <li>
          Prefer a host firewall rule: allow port <code>{port}</code> only from the Boomerang
          IP(s) above.
        </li>
        <li>
          Cloud panels (CloudPanel, ufw, csf) often have an &quot;allow IP&quot; option — use that
          instead of &quot;allow all&quot;.
        </li>
      </ul>
    </aside>
  );
}
