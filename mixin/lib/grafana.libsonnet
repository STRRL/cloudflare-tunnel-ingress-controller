// Small helper library to build Grafana dashboard JSON.
// It only covers the pieces this project needs: dashboards,
// template variables, timeseries panels, and stat panels.
{
  local datasource = { type: 'prometheus', uid: '${datasource}' },

  dashboard(title, uid, panels, variables=[]):: {
    title: title,
    uid: uid,
    schemaVersion: 39,
    editable: true,
    graphTooltip: 1,
    timezone: '',
    refresh: '30s',
    time: { from: 'now-6h', to: 'now' },
    tags: ['cloudflare-tunnel-ingress-controller'],
    templating: {
      list: [
        {
          name: 'datasource',
          label: 'Data source',
          type: 'datasource',
          query: 'prometheus',
        },
      ] + variables,
    },
    panels: panels,
  },

  // A template variable filled from a label of an existing metric.
  queryVariable(name, query, multi=false):: {
    name: name,
    type: 'query',
    datasource: datasource,
    query: { query: query, refId: 'var-' + name },
    refresh: 2,
    sort: 1,
    multi: multi,
    includeAll: multi,
  },

  target(expr, legend):: {
    expr: expr,
    legendFormat: legend,
    datasource: datasource,
  },

  local withRefIds(targets) = std.mapWithIndex(
    function(i, t) t { refId: std.char(std.codepoint('A') + i) },
    targets,
  ),

  timeseries(title, targets, gridPos, unit='short'):: {
    type: 'timeseries',
    title: title,
    datasource: datasource,
    gridPos: gridPos,
    fieldConfig: {
      defaults: {
        unit: unit,
        custom: {
          fillOpacity: 10,
          showPoints: 'never',
          lineWidth: 1,
        },
      },
      overrides: [],
    },
    options: {
      legend: { displayMode: 'list', placement: 'bottom' },
      tooltip: { mode: 'multi', sort: 'desc' },
    },
    targets: withRefIds(targets),
  },

  // A heatmap fed by Prometheus histogram buckets. The query must keep the
  // le label, for example: sum by (le) (increase(some_bucket[interval])).
  heatmap(title, expr, gridPos, unit='s'):: {
    type: 'heatmap',
    title: title,
    datasource: datasource,
    gridPos: gridPos,
    fieldConfig: { defaults: {}, overrides: [] },
    options: {
      calculate: false,
      cellGap: 1,
      color: { mode: 'scheme', scheme: 'Spectral', steps: 64, reverse: true },
      yAxis: { unit: unit },
      tooltip: { mode: 'single' },
      legend: { show: true },
    },
    targets: [{
      expr: expr,
      format: 'heatmap',
      legendFormat: '{{le}}',
      datasource: datasource,
      refId: 'A',
    }],
  },

  stat(title, targets, gridPos, unit='short', thresholds=null):: {
    type: 'stat',
    title: title,
    datasource: datasource,
    gridPos: gridPos,
    fieldConfig: {
      defaults: {
        unit: unit,
        [if thresholds != null then 'thresholds']: thresholds,
      },
      overrides: [],
    },
    options: {
      colorMode: 'value',
      graphMode: 'none',
      reduceOptions: { calcs: ['lastNotNull'] },
    },
    targets: withRefIds(targets),
  },
}
