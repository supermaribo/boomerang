import { Link, useLocation } from "react-router-dom";

type Props = {
  onLogout?: () => void;
};

const links = [
  { to: "/app", label: "Dashboard", end: true },
  { to: "/app/file-servers", label: "File servers" },
  { to: "/app/databases", label: "Databases" },
  { to: "/app/settings", label: "Settings" },
];

export default function Nav({ onLogout }: Props) {
  const loc = useLocation();
  return (
    <nav className="nav">
      <Link to="/app" className="nav-brand">
        Boomerang
      </Link>
      <div className="nav-links">
        {links.map((l) => {
          const active = l.end
            ? loc.pathname === l.to
            : loc.pathname === l.to || loc.pathname.startsWith(l.to + "/");
          return (
            <Link key={l.to} to={l.to} className={active ? "nav-link active" : "nav-link"}>
              {l.label}
            </Link>
          );
        })}
      </div>
      {onLogout && (
        <button type="button" className="ghost nav-out" onClick={onLogout}>
          Sign out
        </button>
      )}
    </nav>
  );
}
