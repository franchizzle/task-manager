import { useState } from 'react'
import { DateTime } from 'luxon'
import { REACT_APP_TASK_BASE_URL, SHARED_ITEM_INDEFINITE_DATE } from '../../constants'
import { useToast } from '../../hooks'
import { useModifyTask } from '../../services/api/tasks.hooks'
import { icons } from '../../styles/images'
import { TTaskSharedAccess, TTaskV4 } from '../../utils/types'
import GTButton from '../atoms/buttons/GTButton'
import GTDropdownMenu from '../radix/GTDropdownMenu'
import { GTMenuItem } from '../radix/RadixUIConstants'

interface TaskharingDropdownProps {
    task: TTaskV4
}

const TaskSharingDropdown = ({ task }: TaskharingDropdownProps) => {
    const [isOpen, setIsOpen] = useState(false)
    const { mutate: modifyTask } = useModifyTask()
    const toast = useToast()

    const copyTaskLink = () => {
        navigator.clipboard.writeText(`${REACT_APP_TASK_BASE_URL}/task/${task.id}`)
        toast.show(
            {
                message: `Task URL copied to clipboard`,
            },
            {
                autoClose: 2000,
                pauseOnFocusLoss: false,
                theme: 'dark',
            }
        )
    }

    const shareAndCopy = (shareAccess: TTaskSharedAccess) => {
        modifyTask({
            id: task.id,
            shared_until: SHARED_ITEM_INDEFINITE_DATE,
            shared_access: shareAccess,
        })
        copyTaskLink()
    }
    const unshareTask = () => {
        modifyTask({
            id: task.id,
            shared_until: DateTime.fromMillis(1).toISO(),
        })
    }

    const isShared = +DateTime.fromISO(task.shared_until ?? '0') > +DateTime.local()

    const sharedDropdownItems: GTMenuItem[] = [
        {
            icon: icons.copy,
            label: 'Copy link',
            hideCheckmark: true,
            onClick: copyTaskLink,
        },
        {
            icon: icons.share,
            label: 'Share task with',
            hideCheckmark: true,
            subItems: [
                {
                    icon: icons.users,
                    label: 'Share with company',
                    hideCheckmark: true,
                    onClick: () => shareAndCopy('domain'),
                },
                {
                    icon: icons.globe,
                    label: 'Share with everyone',
                    hideCheckmark: true,
                    onClick: () => shareAndCopy('public'),
                },
            ],
        },
        {
            icon: icons.link_slashed,
            label: 'Disable shared link',
            hideCheckmark: true,
            onClick: unshareTask,
        },
    ]
    const notSharedDropdownItems: GTMenuItem[] = [
        {
            icon: icons.users,
            label: 'Share with company',
            hideCheckmark: true,
            onClick: () => shareAndCopy('domain'),
        },
        {
            icon: icons.globe,
            label: 'Share with everyone',
            hideCheckmark: true,
            onClick: () => shareAndCopy('public'),
        },
    ]

    return (
        <GTDropdownMenu
            isOpen={isOpen}
            setIsOpen={setIsOpen}
            items={isShared ? sharedDropdownItems : notSharedDropdownItems}
            trigger={<GTButton styleType="secondary" icon={icons.share} value="Share" active={isOpen} />}
            description={!isShared ? 'This task is not being shared.' : undefined}
        />
    )
}

export default TaskSharingDropdown
