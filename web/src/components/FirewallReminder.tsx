import { useEffect, useState } from "react";
import { api } from "../App";

type ApplianceInfo = {
  localIPs?: string[];
  externalIP?: string;
  sourceIPs?: string[];
};

type Props = {
  targetHost?: string;
  port: number;
  protocol: string;
};

export default function FirewallReminder({ targetHost, port, protocol }: Props) {
  const [localIPs, setLocalIPs] = useState<string[]>([]);
  const [externalIP, setExternalIP] = useState("");

  useEffect(() => {
    api<ApplianceInfo>("/api/appliance")
      .then((info) => {
        setLocalIPs(info.localIPs?.length ? info.localIPs : info.sourceIPs || []);
        setExternalIP(info.externalIP || "");
      })
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
      <div className="ip-list">
        {localIPs.length > 0 && (
          <p>
            <strong>LAN / internal:</strong>{" "}
            {localIPs.map((ip, i) => (
              <span key={ip}>
                {i > 0 && ", "}
                <code>{ip}</code>
              </span>
            ))}
          </p>
        )}
        {externalIP && !localIPs.includes(externalIP) && (
          <p>
            <strong>Public / external:</strong> <code>{externalIP}</code>
            <span className="muted small">
              {" "}
              — use this if your sites see a NAT or router address instead of the LAN IP
            </span>
          </p>
        )}
        {localIPs.length === 0 && !externalIP && (
          <p className="muted small">
            Could not detect this server&apos;s IP automatically. Use the address your remote
            firewall sees when Boomerang connects.
          </p>
        )}
      </div>
      <ul className="plain small">
        <li>
          <strong>Do not</strong> open {protocol} to <code>0.0.0.0/0</code> or <code>::/0</code> —
          that exposes your site to the world.
        </li>
        <li>
          Allow port <code>{port}</code> only from the Boomerang IP(s) above (internal and/or
          external).
        </li>
        <li>
          Cloud panels (CloudPanel, ufw, csf) often have an &quot;allow IP&quot; option — use that
          instead of &quot;allow all&quot;.
        </li>
      </ul>
    </aside>
  );
}
