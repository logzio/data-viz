import { css } from '@emotion/css';
import React, { FC, useState } from 'react';
import { useFormContext } from 'react-hook-form';

import { DataSourceInstanceSettings, GrafanaTheme2 } from '@grafana/data';
import { Stack } from '@grafana/experimental';
import { Field, Icon, Input, InputControl, Label, Tooltip, useStyles2 } from '@grafana/ui';
import { contextSrv } from 'app/core/services/context_srv';
import { AccessControlAction } from 'app/types';

import { DashboardSearchHit } from '../../../../search/types';
import { RuleFormType, RuleFormValues } from '../../types/rule-form';

import { CloudRulesSourcePicker } from './CloudRulesSourcePicker';
import { GroupAndNamespaceFields } from './GroupAndNamespaceFields';
import { RuleEditorSection } from './RuleEditorSection';
import { Folder, RuleFolderPicker } from './RuleFolderPicker';
import { RuleTypePicker } from './rule-types/RuleTypePicker';
import { checkForPathSeparator } from './util';

interface Props {
  editingExistingRule: boolean;
}

const recordingRuleNameValidationPattern = {
  message:
    'Recording rule name must be valid metric name. It may only contain letters, numbers, and colons. It may not contain whitespace.',
  value: /^[a-zA-Z_:][a-zA-Z0-9_:]*$/,
};

//LOGZIO GRAFANA CHANGE :: DEV-33636 - Blacklist folder selection options
const blackListedFolderUids = ['logzio-dashboards-folder'];
//LOGZIO GRAFANA CHANGE :: end

export const AlertTypeStep: FC<Props> = ({ editingExistingRule }) => {
  const styles = useStyles2(getStyles);

  const { enabledRuleTypes, defaultRuleType } = getAvailableRuleTypes();

  const {
    register,
    control,
    watch,
    formState: { errors },
    setValue,
    getValues,
  } = useFormContext<RuleFormValues & { location?: string }>();

  // LOGZ.IO CHANGE :: Start
  const [folders, setFolders] = useState<string[]>([]);
  const [newName, setNewName] = useState<string | null>(null);
  const folder = watch('folder');
  const group = watch('group');
  const folderFilter = (hits: DashboardSearchHit[]) => {
    return hits.filter((hit) => !blackListedFolderUids.includes(hit.uid || ''));
  };

  if (group !== '' && newName !== null) {
    if (folders.find((f: string) => f === group) === undefined) {
      setNewName(null);
    }
  }

  if (folder?.title && folders[0] !== folder?.title) {
    setFolders([folder?.title, ...folders]);
  }

  const onFolderChange = (d: { id: number; title: string }) => {
    let grp = group;

    if (group !== '') {
      if (group === d.title) {
        grp = group;
      } else {
        if (folders.find((f: string) => f === group) === undefined) {
          grp = group;
        } else {
          grp = d.title;
        }
      }
    } else {
      grp = d.title;
    }
    setNewName(grp);
    setValue('group', grp);
    setValue('folder', d);
  };
  // LOGZ.IO CHANGE :: Start

  const ruleFormType = watch('type');
  const dataSourceName = watch('dataSourceName');

  return (
    <RuleEditorSection stepNo={1} title="Rule type">
      {!editingExistingRule && (
        <Field error={errors.type?.message} invalid={!!errors.type?.message} data-testid="alert-type-picker">
          <InputControl
            render={({ field: { onChange } }) => (
              <RuleTypePicker
                aria-label="Rule type"
                selected={getValues('type') ?? defaultRuleType}
                onChange={onChange}
                enabledTypes={enabledRuleTypes}
              />
            )}
            name="type"
            control={control}
            rules={{
              required: { value: true, message: 'Please select alert type' },
            }}
          />
        </Field>
      )}

      <Field
        className={styles.formInput}
        label="Rule name"
        error={errors?.name?.message}
        invalid={!!errors.name?.message}
      >
        <Input
          id="name"
          {...register('name', {
            required: { value: true, message: 'Must enter an alert name' },
            pattern: ruleFormType === RuleFormType.cloudRecording ? recordingRuleNameValidationPattern : undefined,
            validate: {
              pathSeparator: (value: string) => {
                // we use the alert rule name as the "groupname" for Grafana managed alerts, so we can't allow path separators
                if (ruleFormType === RuleFormType.grafana) {
                  return checkForPathSeparator(value);
                }

                return true;
              },
            },
          })}
          autoFocus={true}
        />
      </Field>
      <div className={styles.flexRow}>
        {(ruleFormType === RuleFormType.cloudRecording || ruleFormType === RuleFormType.cloudAlerting) && (
          <Field
            className={styles.formInput}
            label="Select data source"
            error={errors.dataSourceName?.message}
            invalid={!!errors.dataSourceName?.message}
            data-testid="datasource-picker"
          >
            <InputControl
              render={({ field: { onChange, ref, ...field } }) => (
                <CloudRulesSourcePicker
                  {...field}
                  onChange={(ds: DataSourceInstanceSettings) => {
                    // reset location if switching data sources, as different rules source will have different groups and namespaces
                    setValue('location', undefined);
                    onChange(ds?.name ?? null);
                  }}
                />
              )}
              name="dataSourceName"
              control={control}
              rules={{
                required: { value: true, message: 'Please select a data source' },
              }}
            />
          </Field>
        )}
      </div>
      {(ruleFormType === RuleFormType.cloudRecording || ruleFormType === RuleFormType.cloudAlerting) &&
        dataSourceName && <GroupAndNamespaceFields rulesSourceName={dataSourceName} />}

      {ruleFormType === RuleFormType.grafana && (
        <div className={styles.flexRow}>
          <Field
            label={
              <Label htmlFor="folder" description={'Select a folder to store your rule.'}>
                <Stack gap={0.5}>
                  Folder
                  <Tooltip
                    placement="top"
                    content={
                      <div>
                        Each folder has unique folder permission. When you store multiple rules in a folder, the folder
                        access permissions get assigned to the rules.
                      </div>
                    }
                  >
                    <Icon name="info-circle" size="xs" />
                  </Tooltip>
                </Stack>
              </Label>
            }
            className={styles.formInput}
            error={errors.folder?.message}
            invalid={!!errors.folder?.message}
            data-testid="folder-picker"
          >
            <InputControl
              render={({ field: { ref, ...field } }) => (
                <RuleFolderPicker
                  inputId="folder"
                  {...field}
                  enableCreateNew={true}
                  enableReset={true}
                  onChange={onFolderChange}
                  filter={folderFilter}
                /> // LOGZ.IO CHANGE
              )}
              name="folder"
              rules={{
                required: { value: true, message: 'Please select a folder' },
                validate: {
                  pathSeparator: (folder: Folder) => checkForPathSeparator(folder.title),
                },
              }}
            />
          </Field>
          <Field
            label="Group"
            data-testid="group-picker"
            description="Allows to group alerts inside the folder. Group name is displayed when using group view" // LOGZ.IO Changes
            className={styles.formInput}
            error={errors.group?.message}
            invalid={!!errors.group?.message}
          >
            <Input
              id="group"
              value={newName ? newName : group} // LOGZ.IO CHANGE :: END
              {...register('group', {
                required: { value: true, message: 'Must enter a group name' },
              })}
            />
          </Field>
        </div>
      )}
    </RuleEditorSection>
  );
};

function getAvailableRuleTypes() {
  const canCreateGrafanaRules = contextSrv.hasPermission(AccessControlAction.AlertingRuleCreate);
  const canCreateCloudRules = contextSrv.hasPermission(AccessControlAction.AlertingRuleExternalWrite);
  const defaultRuleType = canCreateGrafanaRules ? RuleFormType.grafana : RuleFormType.cloudAlerting;

  const enabledRuleTypes: RuleFormType[] = [];
  if (canCreateGrafanaRules) {
    enabledRuleTypes.push(RuleFormType.grafana);
  }
  if (canCreateCloudRules) {
    enabledRuleTypes.push(RuleFormType.cloudAlerting, RuleFormType.cloudRecording);
  }

  return { enabledRuleTypes, defaultRuleType };
}

const getStyles = (theme: GrafanaTheme2) => ({
  formInput: css`
    width: 330px;
    & + & {
      margin-left: ${theme.spacing(3)};
    }
  `,
  flexRow: css`
    display: flex;
    flex-direction: row;
    justify-content: flex-start;
    align-items: flex-end;
  `,
});
