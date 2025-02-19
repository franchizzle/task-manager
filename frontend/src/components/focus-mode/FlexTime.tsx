import { useCallback, useLayoutEffect, useMemo, useState } from 'react'
import { DateTime } from 'luxon'
import styled from 'styled-components'
import { v4 as uuidv4 } from 'uuid'
import { useSetting } from '../../hooks'
import useGetActiveTasks from '../../hooks/useGetActiveTasks'
import { useCreateEvent, useEvents, useGetCalendars } from '../../services/api/events.hooks'
import Log from '../../services/api/log'
import { useGetOverviewViews } from '../../services/api/overview.hooks'
import { Colors, Spacing, Typography } from '../../styles'
import { logos } from '../../styles/images'
import { getMonthsAroundDate } from '../../utils/time'
import { TEvent, TTaskV4 } from '../../utils/types'
import Flex from '../atoms/Flex'
import GTHeader from '../atoms/GTHeader'
import GTTitle from '../atoms/GTTitle'
import { Icon } from '../atoms/Icon'
import { BodyLarge, BodySmallUpper, TitleMedium } from '../atoms/typography/Typography'
import useConnectGoogleAccountToast from '../calendar/utils/useConnectGoogleAccountToast'
import ItemContainer from '../molecules/ItemContainer'

const FlexTimeContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: ${Spacing._32};
`
const TaskSelectionContainer = styled.div`
    display: flex;
    flex-direction: column;
    gap: ${Spacing._24};
`
const RecommendedTasks = styled.div`
    display: flex;
    flex-direction: column;
    gap: ${Spacing._8};
`
const RecommendedTaskContainer = styled.div`
    display: flex;
    padding: ${Spacing._8} 0;
    align-items: center;
    overflow: hidden;
    text-overflow: ellipsis;
`
const TaskTitle = styled.span`
    margin-left: ${Spacing._16};
    overflow: hidden;
    text-overflow: ellipsis;
`
const NewTaskRecommendationsButton = styled.div`
    color: ${Colors.text.purple};
    ${Typography.body.medium};
    cursor: pointer;
    user-select: none;
    width: fit-content;
`

const currentFifteenMinuteBlock = (currentTime: DateTime) => {
    // Round down to nearest 15 minutes
    const minutes = Math.floor(currentTime.minute / 15) * 15
    return DateTime.local().set({ minute: minutes, second: 0, millisecond: 0 })
}

const getRandomUniqueTaskIds = (tasksLength: number): [number?, number?] => {
    if (tasksLength === 0) {
        return [undefined, undefined]
    } else if (tasksLength === 1) {
        return [0, undefined]
    } else if (tasksLength === 2) {
        return [0, 1]
    }
    const taskIds: [number?, number?] = []
    while (taskIds.length < 3) {
        const randomId = Math.floor(Math.random() * tasksLength)
        if (!taskIds.includes(randomId)) {
            taskIds.push(randomId)
        }
    }
    return taskIds
}

const getFlexTimeText = (events: TEvent[], nextEvent?: TEvent) => {
    const latestPriorEvent = events.filter((event) => DateTime.fromISO(event.datetime_end) < DateTime.local()).pop()
    //check if event ends after 11:30AM
    const eventEndedBeforeMorningCutoff =
        latestPriorEvent &&
        DateTime.fromISO(latestPriorEvent.datetime_end).hour < 11 &&
        DateTime.fromISO(latestPriorEvent.datetime_end).minute < 30
    if (!latestPriorEvent && !nextEvent) {
        return 'All day'
    } else if (latestPriorEvent && nextEvent) {
        const formattedStart = DateTime.fromISO(latestPriorEvent.datetime_end).toLocaleString(DateTime.TIME_SIMPLE)
        const formattedEnd = DateTime.fromISO(nextEvent.datetime_start).toLocaleString(DateTime.TIME_SIMPLE)
        return `${formattedStart} - ${formattedEnd}`
    } else if (!nextEvent && eventEndedBeforeMorningCutoff) {
        return 'Starting at 11:30am'
    } else if (nextEvent) {
        const formattedEnd = DateTime.fromISO(nextEvent.datetime_start).toLocaleString(DateTime.TIME_SIMPLE)
        return `Until ${formattedEnd}`
    }
    return 'Until Midnight'
}

interface FlexTimeProps {
    nextEvent?: TEvent
}

const FlexTime = ({ nextEvent }: FlexTimeProps) => {
    const date = DateTime.local()
    const monthBlocks = useMemo(() => {
        const blocks = getMonthsAroundDate(date, 1)
        return blocks.map((block) => ({ startISO: block.start.toISO(), endISO: block.end.toISO() }))
    }, [date])
    const { data: eventsCurrentMonth } = useEvents(monthBlocks[1], 'calendar')
    const todayEvents = eventsCurrentMonth?.filter((event) =>
        DateTime.fromISO(event.datetime_start).hasSame(date, 'day')
    )
    const flexTimeText = getFlexTimeText(todayEvents ?? [], nextEvent)

    const fifteenMinuteBlock = currentFifteenMinuteBlock(DateTime.local())
    const { data: activeTasks } = useGetActiveTasks()
    const { mutate: createEvent } = useCreateEvent()
    const { data: calendars } = useGetCalendars()
    const { data: views } = useGetOverviewViews()
    const { field_value: taskToCalAccount } = useSetting('calendar_account_id_for_new_tasks')
    const { field_value: taskToCalCalendar } = useSetting('calendar_calendar_id_for_new_tasks')
    const showConnectToast = useConnectGoogleAccountToast()

    const [recommendedTasks, setRecommendedTasks] = useState<[TTaskV4?, TTaskV4?]>([])

    const getNewRecommendedTasks = useCallback(() => {
        if (!activeTasks) return
        const [firstId, secondId] = getRandomUniqueTaskIds(activeTasks.length)
        const firstTask = firstId !== undefined ? activeTasks[firstId] : undefined
        const secondTask = secondId !== undefined ? activeTasks[secondId] : undefined
        setRecommendedTasks([firstTask, secondTask])
    }, [activeTasks, views])

    useLayoutEffect(() => {
        if (!recommendedTasks[0] && !recommendedTasks[1]) {
            if (views === undefined) {
                getNewRecommendedTasks()
                return
            }
            const allViewTaskIds = views
                .filter((view) => view.type === 'slack' || view.type === 'task_section' || view.type === 'linear')
                .flatMap((view) => view.view_item_ids)
            const firstTask =
                allViewTaskIds.length > 0 ? activeTasks?.find(({ id }) => allViewTaskIds[0] === id) : undefined
            const secondTask =
                allViewTaskIds.length > 1 ? activeTasks?.find(({ id }) => allViewTaskIds[1] === id) : undefined
            setRecommendedTasks([firstTask, secondTask])
            if (!firstTask && !secondTask) {
                getNewRecommendedTasks()
            }
        }
    }, [activeTasks])

    const onClickHandler = (task: TTaskV4) => {
        if (!calendars?.length) {
            showConnectToast()
            return
        }
        let description = task.body
        if (description !== '') {
            description += '\n'
        }
        description = description.replaceAll('\n', '<br>')
        description += '<a href="https://resonant-kelpie-404a42.netlify.app/" __is_owner="true">created by General Task</a>'
        createEvent({
            createEventPayload: {
                account_id: taskToCalAccount,
                calendar_id: taskToCalCalendar,
                datetime_start: fifteenMinuteBlock.toISO(),
                datetime_end: fifteenMinuteBlock.plus({ hours: 1 }).toISO(),
                summary: task.title,
                description,
                task_id: task.id,
            },
            date: DateTime.local(),
            linkedTask: task,
            optimisticId: uuidv4(),
        })
        Log(`flex_time_create_event_from_task`)
    }

    return (
        <FlexTimeContainer>
            <GTHeader>Flex Time</GTHeader>
            <GTTitle>{flexTimeText}</GTTitle>
            <Flex column gap={Spacing._16}>
                <TitleMedium>Need something to work on?</TitleMedium>
                <BodyLarge>
                    We&apos;ve picked a couple tasks that you may be interested in doing now. Click a task below to add
                    it to your calendar and get started, or have us show you a couple other options to choose from.
                    <br />
                    <br />
                    Remember, you can always schedule tasks by dragging them onto the calendar before entering Focus
                    Mode.
                </BodyLarge>
            </Flex>
            <TaskSelectionContainer>
                <BodySmallUpper>Chosen for you — Click to get started</BodySmallUpper>
                <RecommendedTasks>
                    {recommendedTasks.map(
                        (task) =>
                            task && (
                                <ItemContainer key={task.id} isSelected={false} onClick={() => onClickHandler(task)}>
                                    <RecommendedTaskContainer>
                                        <Icon icon={logos.generaltask} />
                                        <TaskTitle>{task.title}</TaskTitle>
                                    </RecommendedTaskContainer>
                                </ItemContainer>
                            )
                    )}
                </RecommendedTasks>
                <NewTaskRecommendationsButton onClick={getNewRecommendedTasks}>
                    Find me something else to work on
                </NewTaskRecommendationsButton>
            </TaskSelectionContainer>
        </FlexTimeContainer>
    )
}
export default FlexTime
