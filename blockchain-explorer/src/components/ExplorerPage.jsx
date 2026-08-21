import React from "react";

export const ExplorerPageHero = ({
  eyebrow = "PoDL explorer",
  title,
  description,
  metaLabel = "Public testnet",
  metaValue = "Live ledger data",
  children,
}) => (
  <header className="explorer-page-hero">
    <div className="explorer-page-copy">
      <span className="explorer-page-eyebrow">{eyebrow}</span>
      <h1>{title}</h1>
      <p>{description}</p>
      {children && <div className="explorer-page-actions">{children}</div>}
    </div>
    <div className="explorer-page-meta" aria-label={`${metaLabel}: ${metaValue}`}>
      <small>{metaLabel}</small>
      <strong>{metaValue}</strong>
    </div>
  </header>
);

export const MetricStrip = ({ items = [] }) => (
  <section className="explorer-metric-strip" aria-label="Page metrics">
    {items.map((item, index) => (
      <article key={`${item.label}-${index}`}>
        <span>{item.label}</span>
        <strong>{item.value}</strong>
        {item.note && <small>{item.note}</small>}
      </article>
    ))}
  </section>
);

export const DataSurface = ({ title, description, action, children, className = "" }) => (
  <section className={`explorer-data-surface ${className}`.trim()}>
    {(title || description || action) && (
      <div className="explorer-surface-head">
        <div>
          {title && <h2>{title}</h2>}
          {description && <p>{description}</p>}
        </div>
        {action && <div className="explorer-surface-action">{action}</div>}
      </div>
    )}
    {children}
  </section>
);

export const PremiumPagination = ({
  page,
  totalPages,
  pageNumbers,
  start,
  end,
  total,
  label,
  goTo,
}) => (
  <nav className="premium-pagination" aria-label={`${label} pagination`}>
    <p>
      Showing <strong>{total > 0 ? start : 0}–{end}</strong> of <strong>{total.toLocaleString()}</strong> {label}
    </p>
    <div>
      <button type="button" onClick={() => goTo(1)} disabled={page === 1} aria-label="First page">«</button>
      <button type="button" onClick={() => goTo(page - 1)} disabled={page === 1}>Prev</button>
      {pageNumbers.map((number, index, numbers) => (
        <React.Fragment key={number}>
          {index > 0 && numbers[index - 1] !== number - 1 && <span aria-hidden="true">…</span>}
          <button
            type="button"
            className={number === page ? "active" : ""}
            onClick={() => goTo(number)}
            aria-current={number === page ? "page" : undefined}
          >
            {number}
          </button>
        </React.Fragment>
      ))}
      <button type="button" onClick={() => goTo(page + 1)} disabled={page === totalPages}>Next</button>
      <button type="button" onClick={() => goTo(totalPages)} disabled={page === totalPages} aria-label="Last page">»</button>
    </div>
  </nav>
);
