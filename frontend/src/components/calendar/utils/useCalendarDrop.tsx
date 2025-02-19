import { useCallback, useMemo, useState } from 'react'
import { useEffect } from 'react'
import { DropTargetMonitor, useDrop } from 'react-dnd'
import { renderToString } from 'react-dom/server'
import { DateTime } from 'luxon'
import showdown from 'showdown'
import { v4 as uuidv4 } from 'uuid'
import { useSetting } from '../../../hooks'
import { useCreateEvent, useGetCalendars, useModifyEvent } from '../../../services/api/events.hooks'
import { getDiffBetweenISOTimes } from '../../../utils/time'
import { DropItem, DropType, TEvent } from '../../../utils/types'
import adf2md from '../../atoms/GTTextField/AtlassianEditor/adfToMd'
import { NuxTaskBodyStatic } from '../../details/NUXTaskBody'
import {
    CELL_HEIGHT_VALUE,
    EVENT_CREATION_INTERVAL_HEIGHT,
    EVENT_CREATION_INTERVAL_IN_MINUTES,
    EVENT_CREATION_INTERVAL_PER_HOUR,
} from '../CalendarEvents-styles'
import useConnectGoogleAccountToast from './useConnectGoogleAccountToast'

interface CalendarDropArgs {
    date: DateTime
    eventsContainerRef: React.MutableRefObject<HTMLDivElement | null>
}

const useCalendarDrop = ({ date, eventsContainerRef }: CalendarDropArgs) => {
    const { mutate: createEvent } = useCreateEvent()
    const { mutate: modifyEvent } = useModifyEvent()
    const [dropPreviewPosition, setDropPreviewPosition] = useState(0)
    const [eventPreview, setEventPreview] = useState<TEvent>()
    const { data: calendars } = useGetCalendars()
    const { field_value: taskToCalAccount } = useSetting('calendar_account_id_for_new_tasks')
    const { field_value: taskToCalCalendar } = useSetting('calendar_calendar_id_for_new_tasks')
    const showConnectToast = useConnectGoogleAccountToast()

    const getTimeFromDropPosition = useCallback(
        (dropPosition: number) =>
            date.set({
                hour: dropPosition / EVENT_CREATION_INTERVAL_PER_HOUR,
                minute: (dropPosition % EVENT_CREATION_INTERVAL_PER_HOUR) * EVENT_CREATION_INTERVAL_IN_MINUTES,
                second: 0,
                millisecond: 0,
            }),
        [date]
    )

    const eventPreviewAtHoverTime: TEvent | undefined = useMemo(() => {
        if (!eventPreview) return undefined
        const start = getTimeFromDropPosition(dropPreviewPosition)
        const duration = getDiffBetweenISOTimes(eventPreview.datetime_start, eventPreview.datetime_end)
        const end = start.plus(duration)
        return {
            ...eventPreview,
            datetime_start: start.toISO(),
            datetime_end: end.toISO(),
        }
    }, [eventPreview, dropPreviewPosition])

    // returns index of 15 minute block on the calendar, i.e. 12 am is 0, 12:15 AM is 1, etc.
    const getDropPosition = useCallback((monitor: DropTargetMonitor) => {
        const clientOffset = monitor.getClientOffset()
        const itemType = monitor.getItemType()
        // if dragging an event, the distance from the mouse to the top of the event
        let mouseFromEventTopOffset = 0
        if (itemType === DropType.EVENT) {
            const initialClientOffset = monitor.getInitialClientOffset()
            const initialSourceClientOffset = monitor.getInitialSourceClientOffset()
            const { event } = monitor.getItem<DropItem>()
            if (initialClientOffset && initialSourceClientOffset && event) {
                const startTime = DateTime.fromISO(event.datetime_start)
                //Check how many hours are today taking into account DST
                const numberOfHoursToday = Math.ceil(date.endOf('day').diff(date.startOf('day'), 'hours').hours)
                // DST Offset
                const dstOffset = numberOfHoursToday - 24
                const eventBodyTop =
                    CELL_HEIGHT_VALUE * (startTime.diff(startTime.startOf('day'), 'hours').hours - dstOffset)
                mouseFromEventTopOffset = initialClientOffset.y - initialSourceClientOffset.y - eventBodyTop
            }
        }
        // snap drop position to mouse position
        if (itemType === DropType.EVENT || itemType === DropType.EVENT_RESIZE_HANDLE) {
            mouseFromEventTopOffset -= EVENT_CREATION_INTERVAL_HEIGHT / 2
        }
        if (!eventsContainerRef?.current || !clientOffset) return 0
        const eventsContainerOffset = eventsContainerRef.current.getBoundingClientRect().y
        const yPosInEventsContainer =
            clientOffset.y - eventsContainerOffset + eventsContainerRef.current.scrollTop - mouseFromEventTopOffset
        return Math.floor(yPosInEventsContainer / EVENT_CREATION_INTERVAL_HEIGHT)
    }, [])

    const onDrop = useCallback(
        (item: DropItem, monitor: DropTargetMonitor) => {
            const itemType = monitor.getItemType()
            if (!calendars?.length) {
                showConnectToast()
                return
            }
            const dropPosition = getDropPosition(monitor)
            const dropTime = getTimeFromDropPosition(dropPosition)
            switch (itemType) {
                case DropType.WEEK_TASK_TO_CALENDAR_TASK:
                case DropType.SUBTASK:
                case DropType.NON_REORDERABLE_TASK:
                case DropType.DUE_TASK:
                case DropType.TASK:
                case DropType.PULL_REQUEST: {
                    const droppableItem = item.task ?? item.pullRequest
                    if (!droppableItem) return
                    const end = dropTime.plus({ minutes: 30 })
                    const converter = new showdown.Converter()
                    let description = droppableItem.body

                    if (item.task?.id_nux_number) {
                        // if this is a nux task, override body
                        description = renderToString(
                            <NuxTaskBodyStatic nux_number_id={item.task.id_nux_number} renderSettingsModal={false} />
                        )
                    } else {
                        // convert ADF to markdown (if originally ADF)
                        if (item.task?.source.name === 'Jira' && description !== '') {
                            const json = JSON.parse(description)
                            description = adf2md.convert(json).result
                        }
                        // then convert markdown to HTML
                        description = converter.makeHtml(description)
                        if (description !== '') {
                            description += '\n'
                        }
                        description = description.replaceAll('\n', '<br>')
                        description +=
                            '<a href="https://resonant-kelpie-404a42.netlify.app/" __is_owner="true">created by General Task</a>'
                    }

                    createEvent({
                        createEventPayload: {
                            account_id: taskToCalAccount,
                            calendar_id: taskToCalCalendar,
                            datetime_start: dropTime.toISO(),
                            datetime_end: end.toISO(),
                            summary: droppableItem.title,
                            description,
                            task_id: item.task?.id ?? '',
                            pr_id: item.pullRequest?.id ?? '',
                        },
                        date,
                        linkedTask: item.task,
                        linkedPullRequest: item.pullRequest,
                        optimisticId: uuidv4(),
                    })
                    break
                }
                case DropType.EVENT: {
                    if (!item.event) return
                    const end = dropTime.plus(
                        getDiffBetweenISOTimes(item.event.datetime_start, item.event.datetime_end)
                    )
                    modifyEvent(
                        {
                            id: item.event.id,
                            event: item.event,
                            payload: {
                                account_id: item.event.account_id,
                                calendar_id: item.event.calendar_id,
                                datetime_start: dropTime.toISO(),
                                datetime_end: end.toISO(),
                            },
                            date,
                        },
                        item.event.optimisticId
                    )
                    break
                }
                case DropType.EVENT_RESIZE_HANDLE: {
                    if (!item.event) return
                    const eventStart = DateTime.fromISO(item.event.datetime_start)
                    // if end is after start, use drop location, otherwise set to 15 minutes after event started
                    const end =
                        dropTime.diff(eventStart).milliseconds > 0
                            ? dropTime
                            : eventStart.plus({ minutes: EVENT_CREATION_INTERVAL_IN_MINUTES })
                    modifyEvent(
                        {
                            id: item.event.id,
                            event: item.event,
                            payload: {
                                account_id: item.event.account_id,
                                calendar_id: item.event.calendar_id,
                                datetime_end: end.toISO(),
                            },
                            date,
                        },
                        item.event.optimisticId
                    )
                    break
                }
                case DropType.OVERVIEW_VIEW_HEADER: {
                    if (!item.view) return
                    const end = dropTime.plus({ minutes: 30 })
                    createEvent({
                        createEventPayload: {
                            summary: item.view.name,
                            account_id: taskToCalAccount,
                            calendar_id: taskToCalCalendar,
                            datetime_start: dropTime.toISO(),
                            datetime_end: end.toISO(),
                            view_id: item.view.id,
                        },
                        date,
                        linkedView: item.view,
                        optimisticId: uuidv4(),
                    })
                    break
                }
            }
        },
        [date, calendars]
    )

    const [isOver, drop] = useDrop(
        () => ({
            accept: [
                DropType.TASK,
                DropType.SUBTASK,
                DropType.NON_REORDERABLE_TASK,
                DropType.DUE_TASK,
                DropType.EVENT,
                DropType.EVENT_RESIZE_HANDLE,
                DropType.OVERVIEW_VIEW_HEADER,
                DropType.WEEK_TASK_TO_CALENDAR_TASK,
                DropType.PULL_REQUEST,
            ],
            collect: (monitor) => !!calendars?.length && monitor.isOver(),
            drop: onDrop,
            hover: (item, monitor) => {
                const dropPosition = getDropPosition(monitor)
                const itemType = monitor.getItemType()
                switch (itemType) {
                    case DropType.WEEK_TASK_TO_CALENDAR_TASK:
                    case DropType.SUBTASK:
                    case DropType.NON_REORDERABLE_TASK:
                    case DropType.TASK:
                    case DropType.PULL_REQUEST: {
                        setEventPreview(undefined)
                        setDropPreviewPosition(dropPosition)
                        break
                    }
                    case DropType.DUE_TASK: {
                        setEventPreview(undefined)
                        setDropPreviewPosition(dropPosition)
                        break
                    }
                    case DropType.EVENT: {
                        if (!item.event) return
                        setDropPreviewPosition(dropPosition)
                        setEventPreview(item.event)
                        break
                    }
                    case DropType.EVENT_RESIZE_HANDLE: {
                        if (!item.event) return
                        const eventStart = DateTime.fromISO(item.event.datetime_start)
                        // index of 15 minute block of the start time
                        const eventStartPosition =
                            eventStart.diff(date.startOf('day'), 'minutes').minutes / EVENT_CREATION_INTERVAL_IN_MINUTES
                        setDropPreviewPosition(eventStartPosition)
                        const dropTime = getTimeFromDropPosition(dropPosition)
                        const end =
                            dropTime.diff(eventStart).milliseconds > 0
                                ? dropTime
                                : eventStart.plus({ minutes: EVENT_CREATION_INTERVAL_IN_MINUTES })
                        setEventPreview({
                            ...item.event,
                            datetime_end: end.toISO(),
                        })
                        break
                    }
                    case DropType.OVERVIEW_VIEW_HEADER: {
                        setDropPreviewPosition(dropPosition)
                        setEventPreview(item.event)
                        break
                    }
                }
            },
        }),
        [calendars, onDrop, date]
    )

    useEffect(() => {
        drop(eventsContainerRef)
    }, [eventsContainerRef])

    return { isOver, dropPreviewPosition, eventPreview: eventPreviewAtHoverTime }
}

export default useCalendarDrop
