const REPO = "https://github.com/supermaribo/boomerang";
const LICENSE = `${REPO}/blob/main/LICENSE`;

type Props = {
  portal?: boolean;
};

export default function SiteFooter({ portal }: Props) {
  return (
    <footer className={portal ? "site-footer portal" : "site-footer"}>
      <p className="muted small">
        <a href={REPO} target="_blank" rel="noreferrer">
          Boomerang on GitHub
        </a>
        {" · "}
        <a href={LICENSE} target="_blank" rel="noreferrer">
          AGPL-3.0
        </a>
      </p>
      <p className="muted small site-footer-note">
        Free to use and modify. Source must stay open. No proprietary redistribution or resale.
      </p>
    </footer>
  );
}
