import { css, cx } from '@emotion/css';
import TracePageSearchBar from '@jaegertracing/jaeger-ui-components/src/TracePageHeader/TracePageSearchBar';
import { TopOfViewRefType } from '@jaegertracing/jaeger-ui-components/src/TraceTimelineViewer/VirtualizedTraceView';
import React, { useMemo, useState, createRef } from 'react';
import { useAsync } from 'react-use';

import { PanelProps } from '@grafana/data';
import { getDataSourceSrv } from '@grafana/runtime';
import { TraceView } from 'app/features/explore/TraceView/TraceView';
import { useSearch } from 'app/features/explore/TraceView/useSearch';
import { transformDataFrames } from 'app/features/explore/TraceView/utils/transform';

const styles = {
  wrapper: css`
    height: 100%;
    overflow: scroll;
  `,
  container: css`
    width: 120%;
    border: none;
    position: inherit;
  `,
  searBarContainer: css`
    position: -webkit-sticky;
    position: sticky;
    top: 0;
    right: 0;
    z-index: 1000;
    margin-bottom: 20px;
  `,
  body: css`
    padding: 0px 8px 8px 0px;
  `,
};

export const TracesPanel: React.FunctionComponent<PanelProps> = ({ data }) => {
  const topOfViewRef = createRef<HTMLDivElement>();
  const traceProp = useMemo(() => transformDataFrames(data.series[0]), [data.series]);
  const { search, setSearch, spanFindMatches } = useSearch(traceProp?.spans);
  const [focusedSpanIdForSearch, setFocusedSpanIdForSearch] = useState('');
  const [searchBarSuffix, setSearchBarSuffix] = useState('');
  const dataSource = useAsync(async () => {
    return await getDataSourceSrv().get(data.request?.targets[0].datasource?.uid);
  });
  const scrollElement = document.getElementsByClassName(styles.wrapper)[0];
  const containerClasses = cx([styles.container, 'panel-container']);

  if (!data || !data.series.length || !traceProp) {
    return (
      <div className="panel-empty">
        <p>No data found in response</p>
      </div>
    );
  }

  return (
    <div className={styles.wrapper}>
      <div className={containerClasses}>
        <div ref={topOfViewRef}></div>

        <div className={styles.body}>
          {data.series[0]?.meta?.preferredVisualisationType === 'trace' ? (
            <div className={styles.searBarContainer}>
              <TracePageSearchBar
                navigable={true}
                searchValue={search}
                setSearch={setSearch}
                spanFindMatches={spanFindMatches}
                searchBarSuffix={searchBarSuffix}
                setSearchBarSuffix={setSearchBarSuffix}
                focusedSpanIdForSearch={focusedSpanIdForSearch}
                setFocusedSpanIdForSearch={setFocusedSpanIdForSearch}
              />
            </div>
          ) : null}

          <TraceView
            dataFrames={data.series}
            scrollElement={scrollElement}
            traceProp={traceProp}
            spanFindMatches={spanFindMatches}
            search={search}
            focusedSpanIdForSearch={focusedSpanIdForSearch}
            queryResponse={data}
            datasource={dataSource.value}
            topOfViewRef={topOfViewRef}
            topOfViewRefType={TopOfViewRefType.Panel}
          />
        </div>
      </div>
    </div>
  );
};
