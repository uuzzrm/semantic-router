import type { RecipeProbeDetail, RecipeProbeValidationResult } from '../types/recipe'
import styles from './ConfigPageMoMProbesPanel.module.css'

function formatContent(content: unknown): string {
  if (typeof content === 'string') return content
  return JSON.stringify(content, null, 2) ?? String(content ?? '')
}

export function RecipeProbeDetailPanel({ detail }: { detail: RecipeProbeDetail }) {
  return (
    <div className={styles.detailPanel}>
      {detail.notes ? <p className={styles.notes}>{detail.notes}</p> : null}
      {detail.query ? (
        <section>
          <h4>Query</h4>
          <pre>{detail.query}</pre>
        </section>
      ) : null}
      {detail.messages?.length ? (
        <section>
          <h4>Messages</h4>
          <div className={styles.messageList}>
            {detail.messages.map((message, index) => (
              <div key={`${message.role}-${index}`}>
                <strong>{message.role}</strong>
                <pre>{formatContent(message.content)}</pre>
              </div>
            ))}
          </div>
        </section>
      ) : null}
      {detail.tools?.length ? (
        <section>
          <h4>Tools · {detail.tools.length}</h4>
          <pre>{JSON.stringify(detail.tools, null, 2)}</pre>
        </section>
      ) : null}
      {detail.repeat > 1 || detail.padding ? (
        <section>
          <h4>Materialization</h4>
          <dl className={styles.materializationGrid}>
            <div>
              <dt>Query repeat</dt>
              <dd>{detail.repeat || 1}×</dd>
            </div>
            {detail.padding ? (
              <>
                <div>
                  <dt>Padding repeat</dt>
                  <dd>{detail.padding.repeat}×</dd>
                </div>
                <div>
                  <dt>Placement</dt>
                  <dd>{detail.padding.placement}</dd>
                </div>
              </>
            ) : null}
          </dl>
        </section>
      ) : null}
    </div>
  )
}

export function RecipeProbeValidation({ result }: { result: RecipeProbeValidationResult }) {
  const unavailable = Boolean(result.error)
  return (
    <div
      className={`${styles.validationResult} ${result.passed ? styles.validationPass : styles.validationFail}`}
      role={result.passed ? 'status' : 'alert'}
    >
      <div>
        <strong>
          {unavailable
            ? 'Validation unavailable'
            : result.passed
              ? 'Route validated'
              : 'Route mismatch'}
        </strong>
        <span>{result.latency_ms} ms</span>
      </div>
      <dl>
        <div>
          <dt>Expected decision</dt>
          <dd>{result.expected.decision || '—'}</dd>
        </div>
        <div>
          <dt>Actual decision</dt>
          <dd>{result.actual.decision || '—'}</dd>
        </div>
        <div>
          <dt>Actual model</dt>
          <dd>{result.actual.model || '—'}</dd>
        </div>
      </dl>
      {result.failures.length ? (
        <ul>
          {result.failures.map((failure, index) => (
            <li key={`${index}-${failure}`}>{failure}</li>
          ))}
        </ul>
      ) : null}
      {result.error ? <p>{result.error}</p> : null}
    </div>
  )
}
